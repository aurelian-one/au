package commentcmd

import (
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/aurelian-one/au/cmd/au/common"
	"github.com/aurelian-one/au/pkg/au"
)

var Command = &cobra.Command{
	Use:     "comment",
	GroupID: "core",
	Short:   "Create, read, update, and delete comments on the Todos",
	Long:    "Each Todo may have a list of comments associated with it. Comments contain annotations, a log of work, attached images or files.",
}

type marshallableComment struct {
	Id        string     `yaml:"id"`
	CreatedAt time.Time  `yaml:"created_at"`
	CreatedBy string     `yaml:"created_by"`
	UpdatedAt *time.Time `yaml:"updated_at,omitempty"`
	UpdatedBy *string    `yaml:"updated_by,omitempty"`
	MediaType string     `yaml:"media_type"`
	Content   string     `yaml:"content"`
}

func preMarshalComment(comment *au.Comment, snipRaw bool) interface{} {
	out := &marshallableComment{
		Id:        comment.Id,
		CreatedAt: comment.CreatedAt,
		CreatedBy: comment.CreatedBy,
		UpdatedAt: comment.UpdatedAt,
		UpdatedBy: comment.UpdatedBy,
		MediaType: comment.MediaType,
	}
	if comment.MediaType == au.DefaultCommentMediaType {
		out.Content = string(comment.Content)
	} else if snipRaw {
		out.Content = fmt.Sprintf("<%d bytes hidden>", len(comment.Content))
	} else {
		out.Content = base64.RawStdEncoding.EncodeToString(comment.Content)
	}
	return out
}

var getCommand = &cobra.Command{
	Use:        "get <todo-id> <comment-id>",
	Short:      "Get a particular Comment from a Todo by id",
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

		showRaw, err := cmd.Flags().GetBool("raw")
		if err != nil {
			return errors.Wrap(err, "failed to get raw flag")
		}

		encoder := yaml.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent(2)
		return encoder.Encode(preMarshalComment(comment, !showRaw))
	},
}

var listCommand = &cobra.Command{
	Use:        "list <todo-id>",
	Short:      "List Comments on the Todo by id",
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

		preMarshalledComment := make([]interface{}, len(comments))
		for i, c := range comments {
			preMarshalledComment[i] = preMarshalComment(&c, true)
		}

		encoder := yaml.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent(2)
		return encoder.Encode(preMarshalledComment)
	},
}

func readMarkdown(flagValue string, stdin io.Reader) (content []byte, mediaType string, err error) {
	if flagValue == "-" {
		content, err := io.ReadAll(stdin)
		if err != nil {
			return nil, "", errors.Wrap(err, "failed to read standard input")
		} else if len(content) == 0 {
			return nil, "", errors.Wrap(err, "no content available on standard input")
		}
		return content, au.DefaultCommentMediaType, nil
	} else {
		return []byte(flagValue), au.DefaultCommentMediaType, nil
	}
}

func readContent(flagValue string, stdin io.Reader) (content []byte, mediaType string, err error) {
	if flagValue == "-" {
		content, err := io.ReadAll(stdin)
		if err != nil {
			return nil, "", errors.Wrap(err, "failed to read standard input")
		} else if len(content) == 0 {
			return nil, "", errors.Wrap(err, "no content available on standard input")
		}
		return content, "application/octet-stream", nil
	} else {
		content, err := os.ReadFile(flagValue)
		if err != nil {
			return nil, "", errors.Wrap(err, "failed to read file contents")
		}
		mimeSuffix := fmt.Sprintf("; filename=\"%s\"", filepath.Base(flagValue))
		if mt := mime.TypeByExtension(filepath.Ext(flagValue)); mt != "" {
			return content, mt + mimeSuffix, nil
		} else {
			return content, "application/octet-stream" + mimeSuffix, nil
		}
	}
}

var createCommand = &cobra.Command{
	Use:        "create <todo-id>",
	Short:      "Create Comment on the given Todo",
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
		} else if v != "" {
			params.Content, params.MediaType, err = readMarkdown(v, cmd.InOrStdin())
			if err != nil {
				return errors.Wrap(err, "failed to read markdown")
			}
		}

		if v, err := cmd.Flags().GetString("content"); err != nil {
			return errors.Wrap(err, "failed to get file content flag")
		} else if v != "" {
			params.Content, params.MediaType, err = readContent(v, cmd.InOrStdin())
			if err != nil {
				return errors.Wrap(err, "failed to read content")
			}
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

		if v, ok := cmd.Context().Value(common.CurrentAuthorContextKey).(string); ok && v != "" {
			params.CreatedBy = v
		} else if v := ws.Metadata().CurrentAuthor; v != nil {
			params.CreatedBy = *v
		} else {
			return errors.New("no author set, please set one for the current workspace")
		}

		if comment, err := ws.CreateComment(cmd.Context(), cmd.Flags().Arg(0), params); err != nil {
			return err
		} else if err := ws.Flush(); err != nil {
			return errors.Wrap(err, "failed to flush to file")
		} else {
			encoder := yaml.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent(2)
			return encoder.Encode(preMarshalComment(comment, true))
		}
	},
}

var editCommand = &cobra.Command{
	Use:        "edit <todo-id> <comment-id>",
	Short:      "Edit a Comment on a Todo",
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
			params.Content, _, err = readMarkdown(v, cmd.InOrStdin())
			if err != nil {
				return errors.Wrap(err, "failed to read markdown")
			}
		}

		if v, ok := cmd.Context().Value(common.CurrentAuthorContextKey).(string); ok && v != "" {
			params.UpdatedBy = v
		} else if v := ws.Metadata().CurrentAuthor; v != nil {
			params.UpdatedBy = *v
		} else {
			return errors.New("no author set, please set one for the current workspace")
		}

		if v, err := cmd.Flags().GetBool("edit"); err != nil {
			return errors.Wrap(err, "failed to get edit flag")
		} else if v {
			c := string(comment.Content)
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
			return encoder.Encode(preMarshalComment(comment, true))
		}
	},
}

var deleteCommand = &cobra.Command{
	Use:        "delete <todo-id> <comment-id>",
	Short:      "Delete a Comment from a Todo",
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

		var params au.DeleteCommentParams
		if v, ok := cmd.Context().Value(common.CurrentAuthorContextKey).(string); ok && v != "" {
			params.DeletedBy = v
		} else if v := ws.Metadata().CurrentAuthor; v != nil {
			params.DeletedBy = *v
		} else {
			return errors.New("no author set, please set one for the current workspace")
		}

		if err := ws.DeleteComment(cmd.Context(), cmd.Flags().Arg(0), cmd.Flags().Arg(1), params); err != nil {
			return err
		} else if err := ws.Flush(); err != nil {
			return errors.Wrap(err, "failed to flush to file")
		}
		return nil
	},
}

func init() {
	createCommand.Flags().StringP("markdown", "m", "", "Set the markdown content of the comment, or - to read from stdin")
	createCommand.Flags().String("content", "", "Set the content of the comment as raw bytes from a file, or - to indicate standard input")
	createCommand.Flags().Bool("edit", false, "Edit the content using AU_EDITOR")

	editCommand.Flags().StringP("markdown", "m", "", "Set the markdown content of the comment, or - to read from stdin")
	editCommand.Flags().Bool("edit", false, "Edit the content using AU_EDITOR")

	getCommand.Flags().Bool("raw", false, "Show the base64 encoded content for non markdown comments")

	Command.AddCommand(
		getCommand,
		listCommand,
		createCommand,
		editCommand,
		deleteCommand,
	)

	// add some supplimental media types
	_ = mime.AddExtensionType(".md", "text/markdown")
}
