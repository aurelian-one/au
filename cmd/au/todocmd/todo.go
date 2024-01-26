package todocmd

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/automerge/automerge-go"
	"github.com/oklog/ulid/v2"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/aurelian-one/au/cmd/au/common"
	"github.com/aurelian-one/au/pkg/au"
)

var Command = &cobra.Command{
	Use: "todo",
}

var getCommand = &cobra.Command{
	Use: "get",
	Run: func(cmd *cobra.Command, args []string) {
	},
}

func amvToStr(v *automerge.Value, def string) string {
	if v.Kind() == automerge.KindVoid {
		return def
	} else if v.Kind() == automerge.KindText {
		sv, _ := v.Text().Get()
		return sv
	}
	return v.Str()
}

func amvToTime(v *automerge.Value, def time.Time) time.Time {
	if v.Kind() == automerge.KindVoid {
		return def
	}
	return v.Time()
}

var listCommand = &cobra.Command{
	Use:  "list",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := cmd.Context().Value(common.ConfigDirectoryContextKey).(*au.ConfigDirectory)
		if c.CurrentUid == "" {
			return errors.New("no current workspace set")
		}
		raw, err := os.ReadFile(filepath.Join(c.Path, c.CurrentUid+".automerge"))
		if err != nil {
			return errors.Wrap(err, "failed to read workspace file")
		}
		doc, err := automerge.Load(raw)
		if err != nil {
			return errors.Wrap(err, "failed to preview workspace file")
		}

		todos := doc.Path("todos").Map()

		output := make([]map[string]interface{}, 0)
		todoIds, _ := todos.Keys()
		for _, id := range todoIds {
			item, _ := todos.Get(id)
			titleValue, _ := item.Map().Get("title")
			statusValue, _ := item.Map().Get("status")
			createdAtValue, _ := item.Map().Get("created_at")
			descriptionValue, _ := item.Map().Get("description")
			output = append(output, map[string]interface{}{
				"id":          id,
				"title":       amvToStr(titleValue, ""),
				"status":      amvToStr(statusValue, "open"),
				"created_at":  amvToTime(createdAtValue, time.Unix(0, 0)),
				"description": amvToStr(descriptionValue, ""),
			})
		}
		slices.SortFunc(output, func(a, b map[string]interface{}) int {
			aT, bT := a["created_at"].(time.Time), b["created_at"].(time.Time)
			return aT.Compare(bT)
		})
		return yaml.NewEncoder(os.Stdout).Encode(output)
	},
}

var createCommand = &cobra.Command{
	Use:  "create",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := cmd.Context().Value(common.ConfigDirectoryContextKey).(*au.ConfigDirectory)
		if c.CurrentUid == "" {
			return errors.New("no current workspace set")
		}
		raw, err := os.ReadFile(filepath.Join(c.Path, c.CurrentUid+".automerge"))
		if err != nil {
			return errors.Wrap(err, "failed to read workspace file")
		}
		doc, err := automerge.Load(raw)
		if err != nil {
			return errors.Wrap(err, "failed to preview workspace file")
		}

		todos := doc.Path("todos").Map()
		todoUid := ulid.Make().String()

		newTodo := automerge.NewMap()
		if err := todos.Set(todoUid, newTodo); err != nil {
			return errors.Wrap(err, "failed to set todo entry")
		}

		if err := newTodo.Set("status", "open"); err != nil {
			return errors.Wrap(err, "failed to set status")
		} else if err := newTodo.Set("created_at", time.Now().UTC()); err != nil {
			return errors.Wrap(err, "failed to set created_at")
		}

		if v, err := cmd.Flags().GetString("title"); err != nil {
			return errors.Wrap(err, "failed to get title flag")
		} else if v = strings.TrimSpace(v); len(v) < 3 {
			return errors.Wrap(err, "title may not be empty")
		} else if err := newTodo.Set("title", v); err != nil {
			return errors.Wrap(err, "failed to set title")
		}

		if v, err := cmd.Flags().GetString("description"); err != nil {
			return errors.Wrap(err, "failed to get description flag")
		} else if err := newTodo.Set("description", automerge.NewText(v)); err != nil {
			return errors.Wrap(err, "failed to set description")
		}

		if _, err := doc.Commit("added todo " + todoUid); err != nil {
			return errors.Wrap(err, "failed to commit")
		}

		if err := os.WriteFile(filepath.Join(c.Path, c.CurrentUid+".automerge"), doc.Save(), os.FileMode(0600)); err != nil {
			return errors.Wrap(err, "failed to write file")
		}
		return nil
	},
}

var editCommand = &cobra.Command{
	Use:        "edit <uid>",
	Args:       cobra.ExactArgs(1),
	ArgAliases: []string{"uid"},
	RunE: func(cmd *cobra.Command, args []string) error {
		return errors.New("not implemented")
	},
}

var deleteCommand = &cobra.Command{
	Use:        "delete <uid>",
	Args:       cobra.ExactArgs(1),
	ArgAliases: []string{"uid"},
	RunE: func(cmd *cobra.Command, args []string) error {
		c := cmd.Context().Value(common.ConfigDirectoryContextKey).(*au.ConfigDirectory)
		if c.CurrentUid == "" {
			return errors.New("no current workspace set")
		}
		raw, err := os.ReadFile(filepath.Join(c.Path, c.CurrentUid+".automerge"))
		if err != nil {
			return errors.Wrap(err, "failed to read workspace file")
		}
		doc, err := automerge.Load(raw)
		if err != nil {
			return errors.Wrap(err, "failed to preview workspace file")
		}

		todos := doc.Path("todos").Map()
		todoUid := cmd.Flags().Arg(0)
		if err := todos.Delete(todoUid); err != nil {
			return errors.Wrap(err, "failed to delete todo entry")
		}
		if _, err := doc.Commit("removed todo " + todoUid); err != nil {
			return errors.Wrap(err, "failed to commit")
		}
		if err := os.WriteFile(filepath.Join(c.Path, c.CurrentUid+".automerge"), doc.Save(), os.FileMode(0600)); err != nil {
			return errors.Wrap(err, "failed to write file")
		}
		return nil
	},
}

func init() {
	createCommand.Flags().StringP("title", "t", "", "Set the title of the Todo")
	_ = createCommand.MarkFlagRequired("title")
	createCommand.Flags().String("description", "", "Set the description of the Todo")

	Command.AddCommand(
		getCommand,
		listCommand,
		createCommand,
		editCommand,
		deleteCommand,
	)
}
