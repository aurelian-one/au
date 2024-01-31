package todocmd

import (
	"os"
	"slices"

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
	Use:  "get <uid>",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s := cmd.Context().Value(common.StorageContextKey).(au.StorageProvider)
		w := cmd.Context().Value(common.CurrentWorkspaceIdContextKey).(string)
		if w == "" {
			return errors.New("current workspace not set")
		}
		ws, err := s.OpenWorkspace(cmd.Context(), w, false)
		if err != nil {
			return err
		}
		defer ws.Close()

		todo, err := ws.GetTodo(cmd.Context(), cmd.Flags().Arg(0))
		if err != nil {
			return err
		}

		encoder := yaml.NewEncoder(os.Stdout)
		encoder.SetIndent(2)
		return encoder.Encode(todo)
	},
}

var listCommand = &cobra.Command{
	Use:  "list",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		s := cmd.Context().Value(common.StorageContextKey).(au.StorageProvider)
		w := cmd.Context().Value(common.CurrentWorkspaceIdContextKey).(string)
		if w == "" {
			return errors.New("current workspace not set")
		}
		ws, err := s.OpenWorkspace(cmd.Context(), w, false)
		if err != nil {
			return err
		}
		defer ws.Close()

		todos, err := ws.ListTodos(cmd.Context())
		if err != nil {
			return err
		}

		slices.SortFunc(todos, func(a, b au.Todo) int {
			return a.CreatedAt.Compare(b.CreatedAt)
		})
		encoder := yaml.NewEncoder(os.Stdout)
		encoder.SetIndent(2)
		return encoder.Encode(todos)
	},
}

var createCommand = &cobra.Command{
	Use:  "create",
	Args: cobra.NoArgs,
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

		params := au.CreateTodoParams{}

		if v, err := cmd.Flags().GetString("title"); err != nil {
			return errors.Wrap(err, "failed to get title flag")
		} else {
			params.Title = v
		}

		if v, err := cmd.Flags().GetString("description"); err != nil {
			return errors.Wrap(err, "failed to get description flag")
		} else {
			params.Description = v
		}

		if todo, err := ws.CreateTodo(cmd.Context(), params); err != nil {
			return err
		} else if err := ws.Flush(); err != nil {
			return errors.Wrap(err, "failed to flush to file")
		} else {
			encoder := yaml.NewEncoder(os.Stdout)
			encoder.SetIndent(2)
			return encoder.Encode(todo)
		}
	},
}

var editCommand = &cobra.Command{
	Use:        "edit <uid>",
	Args:       cobra.ExactArgs(1),
	ArgAliases: []string{"uid"},
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

		params := au.EditTodoParams{}
		if v, err := cmd.Flags().GetString("title"); err != nil {
			return errors.Wrap(err, "failed to get title flag")
		} else if v != "" {
			params.Title = &v
		}
		if v, err := cmd.Flags().GetString("description"); err != nil {
			return errors.Wrap(err, "failed to get description flag")
		} else if v != "" {
			params.Description = &v
		}
		if v, err := cmd.Flags().GetString("status"); err != nil {
			return errors.Wrap(err, "failed to get status flag")
		} else if v != "" {
			params.Status = &v
		}

		if todo, err := ws.EditTodo(cmd.Context(), cmd.Flags().Arg(0), params); err != nil {
			return err
		} else if err := ws.Flush(); err != nil {
			return errors.Wrap(err, "failed to flush to file")
		} else {
			encoder := yaml.NewEncoder(os.Stdout)
			encoder.SetIndent(2)
			return encoder.Encode(todo)
		}
	},
}

var deleteCommand = &cobra.Command{
	Use:        "delete <uid>",
	Args:       cobra.ExactArgs(1),
	ArgAliases: []string{"uid"},
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

		if err := ws.DeleteTodo(cmd.Context(), cmd.Flags().Arg(0)); err != nil {
			return err
		} else if err := ws.Flush(); err != nil {
			return errors.Wrap(err, "failed to flush to file")
		}
		return nil
	},
}

func init() {
	createCommand.Flags().StringP("title", "t", "", "Set the title of the Todo")
	_ = createCommand.MarkFlagRequired("title")
	createCommand.Flags().String("description", "", "Set the description of the Todo")

	editCommand.Flags().StringP("title", "t", "", "Set the title of the Todo")
	editCommand.Flags().String("description", "", "Set the description of the Todo")
	editCommand.Flags().String("status", "", "Set the status of the Todo")

	Command.AddCommand(
		getCommand,
		listCommand,
		createCommand,
		editCommand,
		deleteCommand,
	)
}
