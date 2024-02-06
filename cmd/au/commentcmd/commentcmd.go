package commentcmd

import (
	"slices"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/aurelian-one/au/cmd/au/common"
	"github.com/aurelian-one/au/pkg/au"
)

var Command = &cobra.Command{
	Use: "comment",
}

var getCommand = &cobra.Command{
	Use:        "get <todo-id> <comment-id>",
	Args:       cobra.ExactArgs(2),
	ArgAliases: []string{"todo-id", "comment-id"},
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

		comment, err := ws.GetComment(cmd.Context(), cmd.Flags().Arg(0), cmd.Flags().Arg(1))
		if err != nil {
			return err
		}
		encoder := yaml.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent(2)
		return encoder.Encode(comment)
	},
}

var listCommand = &cobra.Command{
	Use:        "list <todo-id>",
	Args:       cobra.ExactArgs(1),
	ArgAliases: []string{"todo-id"},
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

		comments, err := ws.ListComments(cmd.Context(), cmd.Flags().Arg(0))
		if err != nil {
			return err
		}

		slices.SortFunc(comments, func(a, b au.Comment) int {
			return a.CreatedAt.Compare(b.CreatedAt)
		})
		encoder := yaml.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent(2)
		return encoder.Encode(comments)
	},
}

var createCommand = &cobra.Command{
	Use:        "create <todo-id>",
	Args:       cobra.ExactArgs(1),
	ArgAliases: []string{"todo-id"},
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

		params := au.CreateCommentParams{}

		if v, err := cmd.Flags().GetString("markdown"); err != nil {
			return errors.Wrap(err, "failed to get markdown content flag")
		} else {
			params.Content = []byte(v)
			params.MediaType = au.DefaultCommentMediaType
		}

		if v, err := cmd.Flags().GetBool("edit"); err != nil {
			return errors.Wrap(err, "failed to get edit flag")
		} else if v {
			c := ""
			if params.Content != nil {
				c = string(params.Content)
			}
			after, err := common.EditContent(cmd.Context(), c)
			if err != nil {
				return err
			}
			params.Content = []byte(after)
		}

		if todo, err := ws.CreateComment(cmd.Context(), cmd.Flags().Arg(0), params); err != nil {
			return err
		} else if err := ws.Flush(); err != nil {
			return errors.Wrap(err, "failed to flush to file")
		} else {
			encoder := yaml.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent(2)
			return encoder.Encode(todo)
		}
	},
}

var editCommand = &cobra.Command{
	Use:        "edit <todo-id> <comment-id>",
	Args:       cobra.ExactArgs(2),
	ArgAliases: []string{"todo-id", "comment-id"},
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

		comment, err := ws.GetComment(cmd.Context(), cmd.Flags().Arg(0), cmd.Flags().Arg(1))
		if err != nil {
			return err
		}

		if comment.MediaType != au.DefaultCommentMediaType {
			return errors.New("cannot edit the content of a non-markdown comment")
		}

		params := au.EditCommentParams{}
		if v, err := cmd.Flags().GetString("markdown"); err != nil {
			return errors.Wrap(err, "failed to get markdown content flag")
		} else if v != "" {
			params.Content = []byte(v)
		}

		if v, err := cmd.Flags().GetBool("edit"); err != nil {
			return errors.Wrap(err, "failed to get edit flag")
		} else if v {
			c := comment.Content
			if params.Content != nil {
				c = string(params.Content)
			}
			after, err := common.EditContent(cmd.Context(), c)
			if err != nil {
				return err
			}
			params.Content = []byte(after)
		}

		if comment, err := ws.EditComment(cmd.Context(), cmd.Flags().Arg(0), cmd.Flags().Arg(1), params); err != nil {
			return err
		} else if err := ws.Flush(); err != nil {
			return errors.Wrap(err, "failed to flush to file")
		} else {
			encoder := yaml.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent(2)
			return encoder.Encode(comment)
		}
	},
}

var deleteCommand = &cobra.Command{
	Use:        "delete <todo-id> <comment-id>",
	Args:       cobra.ExactArgs(2),
	ArgAliases: []string{"todo-id", "comment-id"},
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
		if err := ws.DeleteComment(cmd.Context(), cmd.Flags().Arg(0), cmd.Flags().Arg(1)); err != nil {
			return err
		} else if err := ws.Flush(); err != nil {
			return errors.Wrap(err, "failed to flush to file")
		}
		return nil
	},
}

func init() {
	createCommand.Flags().StringP("markdown", "m", "", "Set the markdown content of the comment")
	createCommand.Flags().Bool("edit", false, "Edit the content using AU_EDITOR")

	editCommand.Flags().StringP("markdown", "m", "", "Set the markdown content of the comment")
	editCommand.Flags().Bool("edit", false, "Edit the content using AU_EDITOR")

	Command.AddCommand(
		getCommand,
		listCommand,
		createCommand,
		editCommand,
		deleteCommand,
	)
}
