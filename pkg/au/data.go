package au

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/automerge/automerge-go"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/oklog/ulid/v2"
	"github.com/pkg/errors"

	"github.com/aurelian-one/au/pkg/auws"
)

type ConfigDirectory struct {
	// Path is the filepath to the config directory, usually the expanded and absolute form of $HOME/.au but it might
	// be a directory local to a target workspace file.
	Path string

	// CurrentUid is the current
	CurrentUid string
}

// Exists returns whether the target config directory exists
func (c *ConfigDirectory) Exists() (bool, error) {
	stat, err := os.Stat(c.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			slog.Debug("config directory does not exist", "dir", c.Path)
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to check directory '%s'", c.Path)
	}
	if !stat.IsDir() {
		return false, errors.Errorf("path '%s' exists but is not a directory", c.Path)
	}
	slog.Debug("config directory exists", "dir", c.Path)
	return true, nil
}

func (c *ConfigDirectory) Init() error {
	if exists, err := c.Exists(); err != nil {
		return err
	} else if !exists {
		slog.Debug("config directory does not exist - creating it", "dir", c.Path)
		return errors.Wrap(os.MkdirAll(c.Path, os.FileMode(0700)), "failed to init config directory")
	}
	slog.Debug("config directory does not exist - asserting permissions", "dir", c.Path)
	return errors.Wrap(os.Chmod(c.Path, os.FileMode(0700)), "failed to set permissions on config directory")
}

func (c *ConfigDirectory) DoesWorkspaceExist(uid string) (bool, error) {
	if stat, err := os.Stat(filepath.Join(c.Path, uid+".automerge")); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, errors.Wrap(err, "failed to check for workspace")
	} else if stat.IsDir() {
		return false, errors.New("path is a directory")
	}
	return true, nil
}

func (c *ConfigDirectory) ChangeCurrentWorkspaceUid(uid string) error {
	if exists, err := c.DoesWorkspaceExist(uid); err != nil {
		return errors.Wrap(err, "failed to check for workspace")
	} else if !exists {
		return errors.New("target workspace does not exist")
	}
	if err := os.Remove(filepath.Join(c.Path, "current")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return errors.Wrap(err, "failed to remove old symlink")
	}
	if err := os.Symlink(filepath.Join(c.Path, uid+".automerge"), filepath.Join(c.Path, "current")); err != nil {
		return errors.Wrap(err, "failed to symlink")
	}
	slog.Debug(fmt.Sprintf("Set new current workspace %s.", uid))
	return nil
}

func (c *ConfigDirectory) ListWorkspaceUids() ([]string, error) {
	entries, err := os.ReadDir(c.Path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list workspaces")
	}
	workspaceUids := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".automerge" {
			name := strings.TrimSuffix(entry.Name(), ".automerge")
			if _, err := ulid.Parse(name); err == nil {
				workspaceUids = append(workspaceUids, name)
			}
		}
	}
	return workspaceUids, nil
}

type WorkspaceMetadata struct {
	Alias     string    `yaml:"alias"`
	Uid       string    `yaml:"uid"`
	CreatedAt time.Time `yaml:"created_at"`
	SizeBytes int       `yaml:"size_bytes"`
	IsCurrent bool      `yaml:"is_current,omitempty"`
}

func (c *ConfigDirectory) GetWorkspaceMetadata(uid string) (*WorkspaceMetadata, error) {
	data, err := os.ReadFile(filepath.Join(c.Path, uid+".automerge"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to read document")
	}
	doc, err := automerge.Load(data)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load document")
	}
	aliasValue, _ := doc.Path("alias").Get()
	createdAtValue, _ := doc.Path("created_at").Get()
	return &WorkspaceMetadata{
		Uid:       uid,
		Alias:     aliasValue.Str(),
		CreatedAt: createdAtValue.Time(),
		SizeBytes: len(data),
		IsCurrent: uid == c.CurrentUid,
	}, nil
}

func (c *ConfigDirectory) CreateNewWorkspace(alias string) (string, error) {
	proposedUid := ulid.Make().String()
	proposedAlias := strings.TrimSpace(alias)
	if len(proposedAlias) == 0 {
		return "", errors.Errorf("The workspace alias cannot be empty")
	}
	workspacePath := filepath.Join(c.Path, proposedUid+".automerge")
	doc := automerge.New()
	_ = doc.Path("alias").Set(proposedAlias)
	_ = doc.Path("created_at").Set(time.Now().UTC())
	if err := doc.Path("todos").Set(&automerge.Map{}); err != nil {
		return "", errors.Wrap(err, "failed to setup todos map")
	}
	changeHash, err := doc.Commit("Init", automerge.CommitOptions{AllowEmpty: true})
	if err != nil {
		return "", errors.Wrap(err, "failed to commit")
	}
	saved := doc.Save()
	if err := os.WriteFile(workspacePath, saved, os.FileMode(0600)); err != nil {
		return "", errors.Wrap(err, "failed to write")
	}
	if err := c.ChangeCurrentWorkspaceUid(proposedUid); err != nil {
		return "", err
	}
	slog.Debug(fmt.Sprintf("Initialised new workspace '%s' (%s@%s) and set it as default.", proposedAlias, proposedUid, changeHash))
	return proposedUid, nil
}

func (c *ConfigDirectory) DeleteWorkspace(uid string) error {
	if c.CurrentUid == uid {
		if err := os.Remove(filepath.Join(c.Path, "current")); err != nil {
			return errors.Wrap(err, "failed to remove symlink")
		}
	}
	if err := os.Remove(filepath.Join(c.Path, uid+".automerge")); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to remove workspace")
	}
	slog.Debug(fmt.Sprintf("Deleted workspace %s.", uid))
	return nil
}

func (c *ConfigDirectory) ListenAndServe(ctx context.Context, address string) error {
	m := mux.NewRouter()
	m.Handle("/workspaces/{uid}/actions/sync", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		uid := mux.Vars(request)["uid"]
		raw, err := os.ReadFile(filepath.Join(c.Path, uid+".automerge"))
		if err != nil {
			slog.Error("failed to read workspace file", "err", err)
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		doc, err := automerge.Load(raw)
		if err != nil {
			slog.Error("failed to load workspace file", "err", err)
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
		if err := auws.Sync(request.Context(), slog.Default(), conn, doc, false); err != nil {
			slog.Error("failed to sync", "err", err)
			_ = conn.Close()
		}
	})).Methods(http.MethodGet)
	m.Handle("/workspaces/{uid}/raw", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		uid := mux.Vars(request)["uid"]
		raw, err := os.ReadFile(filepath.Join(c.Path, uid+".automerge"))
		if err != nil {
			slog.Error("failed to read workspace file", "err", err)
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		doc, err := automerge.Load(raw)
		if err != nil {
			slog.Error("failed to load workspace file", "err", err)
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = writer.Write(doc.Save())
	})).Methods(http.MethodGet)
	server := http.Server{Addr: address, Handler: m}
	go func() {
		<-ctx.Done()
		_ = server.Close()
	}()
	return server.ListenAndServe()
}

func (c *ConfigDirectory) ConnectAndSync(ctx context.Context, uid string, address string) error {
	baseUrl, err := url.Parse(address)
	if err != nil {
		return errors.Wrap(err, "invalid url")
	}
	raw, err := os.ReadFile(filepath.Join(c.Path, uid+".automerge"))
	if err != nil {
		return errors.Wrap(err, "failed to read workspace file")
	}
	doc, err := automerge.Load(raw)
	if err != nil {
		return errors.Wrap(err, "failed to preview workspace file")
	}

	baseUrl.Scheme = "ws"
	baseUrl.RawQuery = ""
	baseUrl.RawFragment = ""
	baseUrl = baseUrl.JoinPath("workspaces", uid, "actions", "sync")
	conn, _, err := websocket.DefaultDialer.Dial(baseUrl.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to dial: %w", err)
	}
	defer conn.Close()

	if err := auws.Sync(ctx, slog.Default(), conn, doc, true); err != nil {
		return fmt.Errorf("failed to sync: %w", err)
	}
	if err := os.WriteFile(filepath.Join(c.Path, uid+".automerge"), doc.Save(), os.FileMode(0600)); err != nil {
		return errors.Wrap(err, "failed to write destination file")
	}
	return nil
}

func (c *ConfigDirectory) ConnectAndImport(ctx context.Context, uid string, address string) error {
	if ok, err := c.DoesWorkspaceExist(uid); err != nil {
		return errors.Wrap(err, "failed to check workspace")
	} else if ok {
		return errors.Wrap(err, "workspace already exists - did you mean to sync instead?")
	}

	baseUrl, err := url.Parse(address)
	if err != nil {
		return errors.Wrap(err, "invalid url")
	}
	baseUrl.RawQuery = ""
	baseUrl.RawFragment = ""
	baseUrl = baseUrl.JoinPath("workspaces", uid, "raw")

	resp, err := http.DefaultClient.Get(baseUrl.String())
	if err != nil {
		return fmt.Errorf("failed to request: %w", err)
	}
	defer resp.Body.Close()

	f, err := os.OpenFile(filepath.Join(c.Path, uid+".automerge"), os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return errors.Wrap(err, "failed to open destination file")
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		return errors.Wrap(err, "failed to write destination file")
	}
	return nil
}
