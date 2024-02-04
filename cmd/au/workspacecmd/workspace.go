package workspacecmd

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"sync/atomic"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/aurelian-one/au/cmd/au/common"
	"github.com/aurelian-one/au/pkg/au"
	"github.com/aurelian-one/au/pkg/auws"
)

//go:generate go run github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen --config=oapi-codegen.cfg.yaml ../../../specification/openapi.yaml

var Command = &cobra.Command{
	Use: "workspace",
}

var initCommand = &cobra.Command{
	Use:        "init <alias>",
	Args:       cobra.ExactArgs(1),
	ArgAliases: []string{"alias"},
	RunE: func(cmd *cobra.Command, args []string) error {
		s := cmd.Context().Value(common.StorageContextKey).(au.StorageProvider)
		w := cmd.Context().Value(common.CurrentWorkspaceIdContextKey).(string)
		metadata, err := s.CreateWorkspace(cmd.Context(), au.CreateWorkspaceParams{Alias: cmd.Flags().Arg(0)})
		if err != nil {
			return err
		}
		if w == "" {
			if err := s.SetCurrentWorkspace(cmd.Context(), metadata.Id); err != nil {
				return errors.Wrap(err, "failed to set new workspace as current")
			}
		}
		encoder := yaml.NewEncoder(cmd.OutOrStdout())
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
		encoder := yaml.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent(2)
		return encoder.Encode(metadataList)
	},
}

var getCommand = &cobra.Command{
	Use:  "get",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		s := cmd.Context().Value(common.StorageContextKey).(au.StorageProvider)
		w := cmd.Context().Value(common.CurrentWorkspaceIdContextKey).(string)
		if w == "" {
			return errors.New("current workspace not set")
		}
		if meta, err := s.GetWorkspace(cmd.Context(), w); err != nil {
			return err
		} else {
			encoder := yaml.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent(2)
			return encoder.Encode(meta)
		}
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
			encoder := yaml.NewEncoder(cmd.OutOrStdout())
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
		server := echo.New()
		server.HideBanner = true
		server.HidePort = true
		server.Use(embedEchoContextMiddleware)
		RegisterHandlers(server, NewStrictHandler(&workspaceServerImpl{Storage: s}, []StrictMiddlewareFunc{}))
		go func() {
			<-cmd.Context().Done()
			_ = server.Shutdown(cmd.Context())
		}()
		listener, err := net.Listen("tcp", cmd.Flags().Arg(0))
		if err != nil {
			return errors.Wrap(err, "failed to listen")
		}
		slog.Info("listening", "addr", listener.Addr().String())
		listenRef, ok := cmd.Context().Value(common.ListenerRefContextKey).(*atomic.Value)
		if ok {
			listenRef.Store(listener)
		}
		server.Listener = listener
		if err := server.Start(listener.Addr().String()); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	},
}

var syncClientCommand = &cobra.Command{
	Use:        "sync <http://localhost:80>",
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

		req, err := NewSynchroniseWorkspaceDocumentRequest(cmd.Flags().Arg(0), w)
		if err != nil {
			return errors.Wrap(err, "failed to create request")
		}
		// workaround some weird gorilla mux behavior
		if req.URL.Scheme == "http" {
			req.URL.Scheme = "ws"
		} else if req.URL.Scheme == "https" {
			req.URL.Scheme = "wss"
		}

		conn, _, err := websocket.DefaultDialer.Dial(req.URL.String(), nil)
		if err != nil {
			return fmt.Errorf("failed to dial: %w", err)
		}
		defer conn.Close()

		if err := auws.Sync(cmd.Context(), slog.Default(), conn, dws.GetDoc(), true); err != nil {
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
	ArgAliases: []string{"address", "id"},
	RunE: func(cmd *cobra.Command, args []string) error {
		s := cmd.Context().Value(common.StorageContextKey).(au.StorageProvider)

		if _, err := s.GetWorkspace(cmd.Context(), cmd.Flags().Arg(1)); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return err
			}
		} else {
			return errors.New("workspace already exists - did you mean to sync instead?")
		}

		c, err := NewClientWithResponses(cmd.Flags().Arg(0))
		if err != nil {
			return errors.Wrap(err, "failed to create client")
		}

		if resp, err := c.DownloadWorkspaceDocumentWithResponse(cmd.Context(), cmd.Flags().Arg(1)); err != nil {
			return fmt.Errorf("failed to request: %w", err)
		} else if resp.StatusCode() == http.StatusOK {
			if metadata, err := s.ImportWorkspace(cmd.Context(), cmd.Flags().Arg(1), resp.Body); err != nil {
				return err
			} else {
				encoder := yaml.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent(2)
				return encoder.Encode(metadata)
			}
		} else {
			return errors.Errorf("non-200 repsonsen code from download api: %d %s", resp.StatusCode(), string(resp.Body))
		}
	},
}

func init() {
	Command.AddCommand(
		initCommand,
		getCommand,
		listCommand,
		useCommand,
		deleteCommand,
		syncServerCommand,
		syncClientCommand,
		syncImportCommand,
	)
}

func embedEchoContextMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.SetRequest(c.Request().WithContext(context.WithValue(c.Request().Context(), "echo", c)))
		return next(c)
	}
}

type workspaceServerImpl struct {
	Storage au.StorageProvider
}

func (w *workspaceServerImpl) ListWorkspace(ctx context.Context, request ListWorkspaceRequestObject) (ListWorkspaceResponseObject, error) {
	if wsList, err := w.Storage.ListWorkspaces(ctx); err != nil {
		return nil, err
	} else {
		output := make([]Workspace, len(wsList))
		for i, ws := range wsList {
			output[i] = Workspace{
				Id:          ws.Id,
				Alias:       ws.Alias,
				CreatedAt:   ws.CreatedAt,
				SizeInBytes: int(ws.SizeBytes),
			}
		}
		return ListWorkspace200JSONResponse(output), nil
	}
}

func (w *workspaceServerImpl) GetWorkspace(ctx context.Context, request GetWorkspaceRequestObject) (GetWorkspaceResponseObject, error) {
	if ws, err := w.Storage.GetWorkspace(ctx, request.Id); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return GetWorkspace404JSONResponse{StandardNotFoundProblemJSONResponse{
				Status: http.StatusNotFound,
			}}, nil
		}
		return nil, err
	} else {
		return GetWorkspace200JSONResponse{
			Id:          ws.Id,
			Alias:       ws.Alias,
			CreatedAt:   ws.CreatedAt,
			SizeInBytes: int(ws.SizeBytes),
		}, nil
	}
}

func (w *workspaceServerImpl) SynchroniseWorkspaceDocument(ctx context.Context, request SynchroniseWorkspaceDocumentRequestObject) (SynchroniseWorkspaceDocumentResponseObject, error) {
	if ws, err := w.Storage.OpenWorkspace(ctx, request.Id, true); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return SynchroniseWorkspaceDocument404JSONResponse{StandardNotFoundProblemJSONResponse{
				Status: http.StatusNotFound,
			}}, nil
		}
		return nil, err
	} else {
		dws, ok := ws.(au.DocProvider)
		if !ok {
			return nil, errors.New("not a doc provider")
		}

		c := ctx.Value("echo").(echo.Context)
		upgrader := websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		}
		conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
		if err != nil {
			return nil, err
		}
		defer conn.Close()
		if err := auws.Sync(ctx, slog.Default(), conn, dws.GetDoc(), false); err != nil {
			return nil, errors.Wrap(err, "failed to sync")
		}
		return nil, nil
	}
}

func (w *workspaceServerImpl) DownloadWorkspaceDocument(ctx context.Context, request DownloadWorkspaceDocumentRequestObject) (DownloadWorkspaceDocumentResponseObject, error) {
	if ws, err := w.Storage.OpenWorkspace(ctx, request.Id, false); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DownloadWorkspaceDocument404JSONResponse{StandardNotFoundProblemJSONResponse{
				Status: http.StatusNotFound,
			}}, nil
		}
		return nil, err
	} else {
		dws, ok := ws.(au.DocProvider)
		if !ok {
			return nil, errors.New("not a doc provider")
		}
		saved := dws.GetDoc().Save()
		return DownloadWorkspaceDocument200ApplicationoctetStreamResponse{
			Body:          bytes.NewReader(saved),
			ContentLength: int64(len(saved)),
		}, nil
	}
}
