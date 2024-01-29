package workspacecmd

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/aurelian-one/au/cmd/au/common"
	"github.com/aurelian-one/au/pkg/au"
	"github.com/aurelian-one/au/pkg/auws"
)

var Command = &cobra.Command{
	Use: "workspace",
}

var initCommand = &cobra.Command{
	Use:        "init <alias>",
	Args:       cobra.ExactArgs(1),
	ArgAliases: []string{"alias"},
	RunE: func(cmd *cobra.Command, args []string) error {
		s := cmd.Context().Value(common.StorageContextKey).(au.StorageProvider)
		metadata, err := s.CreateWorkspace(cmd.Context(), au.CreateWorkspaceParams{Alias: cmd.Flags().Arg(0)})
		if err != nil {
			return err
		}
		encoder := yaml.NewEncoder(os.Stdout)
		encoder.SetIndent(2)
		return encoder.Encode(metadata)
	},
}

var listCommand = &cobra.Command{
	Use:  "list",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		s := cmd.Context().Value(common.StorageContextKey).(au.StorageProvider)
		metadataList, err := s.ListWorkspaces(cmd.Context())
		if err != nil {
			return err
		}
		encoder := yaml.NewEncoder(os.Stdout)
		encoder.SetIndent(2)
		return encoder.Encode(metadataList)
	},
}

var useCommand = &cobra.Command{
	Use:        "use <uid>",
	Args:       cobra.ExactArgs(1),
	ArgAliases: []string{"uid"},
	RunE: func(cmd *cobra.Command, args []string) error {
		s := cmd.Context().Value(common.StorageContextKey).(au.StorageProvider)
		if metadata, err := s.GetWorkspace(cmd.Context(), cmd.Flags().Arg(0)); err != nil {
			return err
		} else {
			if err := s.SetCurrentWorkspace(cmd.Context(), cmd.Flags().Arg(0)); err != nil {
				return err
			}
			encoder := yaml.NewEncoder(os.Stdout)
			encoder.SetIndent(2)
			return encoder.Encode(metadata)
		}
	},
}

var deleteCommand = &cobra.Command{
	Use:        "delete <uid>",
	Args:       cobra.ExactArgs(1),
	ArgAliases: []string{"uid"},
	RunE: func(cmd *cobra.Command, args []string) error {
		s := cmd.Context().Value(common.StorageContextKey).(au.StorageProvider)
		if _, err := s.GetWorkspace(cmd.Context(), cmd.Flags().Arg(0)); err != nil {
			return err
		} else {
			if id, err := s.GetCurrentWorkspace(cmd.Context()); err != nil {
				return err
			} else if id == cmd.Flags().Arg(0) {
				if err := s.SetCurrentWorkspace(cmd.Context(), ""); err != nil {
					return err
				}
			}
			if err := s.DeleteWorkspace(cmd.Context(), cmd.Flags().Arg(0)); err != nil {
				return err
			}
		}
		return nil
	},
}

var syncServerCommand = &cobra.Command{
	Use:        "serve <localhost:80>",
	Args:       cobra.ExactArgs(1),
	ArgAliases: []string{"address"},
	RunE: func(cmd *cobra.Command, args []string) error {
		s := cmd.Context().Value(common.StorageContextKey).(au.StorageProvider)

		m := mux.NewRouter()
		m.Handle("/workspaces/{uid}/actions/sync", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {

			// TODO: this works but doesn't provide concurrency. ideally we should open the workspace in memory in some
			// 	kind of cache so that concurrent requests can consume it.

			uid := mux.Vars(request)["uid"]
			ws, err := s.OpenWorkspace(cmd.Context(), uid, true)
			if err != nil {
				writer.WriteHeader(http.StatusInternalServerError)
				return
			}
			defer ws.Close()
			defer ws.Flush()

			dws, ok := ws.(au.DocProvider)
			if !ok {
				writer.WriteHeader(http.StatusInternalServerError)
				return
			}
			upgrader := websocket.Upgrader{
				ReadBufferSize:  1024,
				WriteBufferSize: 1024,
			}
			conn, err := upgrader.Upgrade(writer, request, nil)
			if err != nil {
				slog.Error("failed to upgrade", "err", err)
				return
			}
			defer conn.Close()
			if err := auws.Sync(request.Context(), slog.Default(), conn, dws.Doc(), false); err != nil {
				slog.Error("failed to sync", "err", err)
				_ = conn.Close()
			}
		})).Methods(http.MethodGet)
		m.Handle("/workspaces/{uid}/raw", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			uid := mux.Vars(request)["uid"]

			ws, err := s.OpenWorkspace(cmd.Context(), uid, false)
			if err != nil {
				writer.WriteHeader(http.StatusInternalServerError)
				return
			}
			defer ws.Close()

			dws, ok := ws.(au.DocProvider)
			if !ok {
				writer.WriteHeader(http.StatusInternalServerError)
				return
			}
			_, _ = writer.Write(dws.Doc().Save())
		})).Methods(http.MethodGet)
		server := http.Server{Addr: cmd.Flags().Arg(0), Handler: m}
		go func() {
			<-cmd.Context().Done()
			_ = server.Close()
		}()
		return server.ListenAndServe()
	},
}

var syncClientCommand = &cobra.Command{
	Use:        "sync <ws://localhost:80>",
	Args:       cobra.ExactArgs(1),
	ArgAliases: []string{"address"},
	RunE: func(cmd *cobra.Command, args []string) error {
		s := cmd.Context().Value(common.StorageContextKey).(au.StorageProvider)
		w := cmd.Context().Value(common.CurrentWorkspaceIdContextKey).(string)
		if w == "" {
			return errors.New("current workspace not set")
		}
		ws, err := s.OpenWorkspace(cmd.Context(), w, true)
		if err != nil {
			return err
		}
		defer ws.Close()
		dws, ok := ws.(au.DocProvider)
		if !ok {
			return errors.New("no doc available")
		}

		baseUrl, err := url.Parse(cmd.Flags().Arg(0))
		if err != nil {
			return errors.Wrap(err, "invalid url")
		}
		baseUrl.Scheme = "ws"
		baseUrl.RawQuery = ""
		baseUrl.RawFragment = ""
		baseUrl = baseUrl.JoinPath("workspaces", w, "actions", "sync")
		conn, _, err := websocket.DefaultDialer.Dial(baseUrl.String(), nil)
		if err != nil {
			return fmt.Errorf("failed to dial: %w", err)
		}
		defer conn.Close()

		if err := auws.Sync(cmd.Context(), slog.Default(), conn, dws.Doc(), true); err != nil {
			return fmt.Errorf("failed to sync: %w", err)
		}
		if err := ws.Flush(); err != nil {
			return errors.Wrap(err, "failed to write destination file")
		}
		return nil
	},
}

var syncImportCommand = &cobra.Command{
	Use:        "sync-import <http://localhost:80>",
	Args:       cobra.ExactArgs(2),
	ArgAliases: []string{"address", "uid"},
	RunE: func(cmd *cobra.Command, args []string) error {
		return errors.New("not implemented")
	},
}

func init() {
	Command.AddCommand(
		initCommand,
		listCommand,
		useCommand,
		deleteCommand,
		syncServerCommand,
		syncClientCommand,
		syncImportCommand,
	)
}
