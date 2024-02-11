package todocmd

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/aurelian-one/au/cmd/au/common"
	"github.com/aurelian-one/au/pkg/au"
)

var Command = &cobra.Command{
	Use:     "todo",
	GroupID: "core",
	Short:   "Create, read, update, and delete Todos",
}

type marshallableTodo struct {
	Id           string     `yaml:"id"`
	CreatedAt    time.Time  `yaml:"created_at"`
	CreatedBy    string     `yaml:"created_by,omitempty"`
	UpdatedAt    *time.Time `yaml:"updated_at,omitempty"`
	UpdatedBy    *string    `yaml:"updated_by,omitempty"`
	CommentCount int        `yaml:"comment_count"`

	Title       string            `yaml:"title"`
	Description string            `yaml:"description,omitempty"`
	Status      string            `yaml:"status"`
	Annotations map[string]string `yaml:"annotations,omitempty"`
}

func preMarshalTodo(todo *au.Todo) interface{} {
	return &marshallableTodo{
		Id:           todo.Id,
		CreatedAt:    todo.CreatedAt,
		CreatedBy:    todo.CreatedBy,
		UpdatedAt:    todo.UpdatedAt,
		UpdatedBy:    todo.UpdatedBy,
		CommentCount: todo.CommentCount,
		Title:        todo.Title,
		Description:  todo.Description,
		Status:       todo.Status,
		Annotations:  todo.Annotations,
	}
}

var getCommand = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a Todo by id",
	Args:  cobra.ExactArgs(1),
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

		encoder := yaml.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent(2)
		return encoder.Encode(preMarshalTodo(todo))
	},
}

var listCommand = &cobra.Command{
	Use:   "list",
	Short: "List all Todos",
	Args:  cobra.NoArgs,
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
		slices.SortStableFunc(todos, func(a, b au.Todo) int {
			rankA, _ := strconv.Atoi(a.Annotations[au.AurelianRankAnnotation])
			rankB, _ := strconv.Atoi(b.Annotations[au.AurelianRankAnnotation])
			return rankB - rankA
		})

		preMashalledTodos := make([]interface{}, len(todos))
		for i, t := range todos {
			preMashalledTodos[i] = preMarshalTodo(&t)
		}

		encoder := yaml.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent(2)
		return encoder.Encode(preMashalledTodos)
	},
}

var createCommand = &cobra.Command{
	Use:   "create",
	Short: "Create a new Todo",
	Args:  cobra.NoArgs,
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

		if v, err := cmd.Flags().GetBool("edit"); err != nil {
			return errors.Wrap(err, "failed to get edit flag")
		} else if v {
			t, d, err := common.EditTitleAndDescription(cmd.Context(), params.Title, params.Description)
			if err != nil {
				return err
			}
			params.Title = t
			params.Description = d
		}

		if v, err := cmd.Flags().GetStringArray("annotation"); err != nil {
			return errors.Wrap(err, "failed to get annotations flag")
		} else {
			params.Annotations = make(map[string]string)
			for _, entry := range v {
				parts := strings.SplitN(entry, "=", 2)
				if len(parts) == 1 {
					return errors.Errorf("invalid annotation argument '%s', must end in = or =value", entry)
				} else if parts[1] == "" {
					return errors.New("cannot set an annotation to an empty string")
				} else {
					params.Annotations[parts[0]] = parts[1]
				}
			}
		}

		if v, ok := cmd.Context().Value(common.CurrentAuthorContextKey).(string); ok && v != "" {
			params.CreatedBy = v
		} else {
			return errors.New("no author set, please set one for the current workspace")
		}

		if todo, err := ws.CreateTodo(cmd.Context(), params); err != nil {
			return err
		} else if err := ws.Flush(); err != nil {
			return errors.Wrap(err, "failed to flush to file")
		} else {
			encoder := yaml.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent(2)
			return encoder.Encode(preMarshalTodo(todo))
		}
	},
}

var editCommand = &cobra.Command{
	Use:        "edit <id>",
	Short:      "Edit a Todo by id",
	Args:       cobra.ExactArgs(1),
	ArgAliases: []string{"id"},
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

		todo, err := ws.GetTodo(cmd.Context(), cmd.Flags().Arg(0))
		if err != nil {
			return err
		}

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

		if v, err := cmd.Flags().GetBool("edit"); err != nil {
			return errors.Wrap(err, "failed to get edit flag")
		} else if v {
			tt, dd := todo.Title, todo.Description
			if params.Title != nil {
				tt = *params.Title
			}
			if params.Description != nil {
				dd = *params.Description
			}
			t, d, err := common.EditTitleAndDescription(cmd.Context(), tt, dd)
			if err != nil {
				return err
			}
			params.Title = &t
			params.Description = &d
		}

		if v, err := cmd.Flags().GetStringArray("annotation"); err != nil {
			return errors.Wrap(err, "failed to get annotations flag")
		} else {
			params.Annotations = make(map[string]string)
			for _, entry := range v {
				parts := strings.SplitN(entry, "=", 2)
				if len(parts) == 1 {
					return errors.Errorf("invalid annotation argument '%s', must end in = or =value", entry)
				} else {
					params.Annotations[parts[0]] = parts[1]
				}
			}
		}

		if v, ok := cmd.Context().Value(common.CurrentAuthorContextKey).(string); ok && v != "" {
			params.UpdatedBy = v
		} else {
			return errors.New("no author set, please set one for the current workspace")
		}

		if todo, err := ws.EditTodo(cmd.Context(), cmd.Flags().Arg(0), params); err != nil {
			return err
		} else if err := ws.Flush(); err != nil {
			return errors.Wrap(err, "failed to flush to file")
		} else {
			encoder := yaml.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent(2)
			return encoder.Encode(preMarshalTodo(todo))
		}
	},
}

var deleteCommand = &cobra.Command{
	Use:        "delete <id>",
	Short:      "Delete a Todo by id",
	Args:       cobra.ExactArgs(1),
	ArgAliases: []string{"id"},
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
	createCommand.Flags().String("description", "", "Set the description of the Todo")
	createCommand.Flags().Bool("edit", false, "Edit the title and description using AU_EDITOR")
	createCommand.Flags().StringArray("annotation", []string{}, "Set an annotation using key=value syntax")
	createCommand.Flags().String("author", "", "Set the author of the Todo as 'Name <email>'")

	editCommand.Flags().StringP("title", "t", "", "Set the title of the Todo")
	editCommand.Flags().String("description", "", "Set the description of the Todo")
	editCommand.Flags().String("status", "", "Set the status of the Todo")
	editCommand.Flags().Bool("edit", false, "Edit the title and description using AU_EDITOR")
	editCommand.Flags().StringArray("annotation", []string{}, "Set an annotation using key=value or clear an annotation using key=")
	editCommand.Flags().String("author", "", "Set the author of the Todo update as 'Name <email>'")

	Command.AddCommand(
		getCommand,
		listCommand,
		createCommand,
		editCommand,
		deleteCommand,
	)
}
