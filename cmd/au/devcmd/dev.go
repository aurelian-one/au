package devcmd

import (
	"encoding/base64"
	"fmt"
	"hash/crc32"
	"os"
	"time"

	"github.com/automerge/automerge-go"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/aurelian-one/au/cmd/au/common"
	"github.com/aurelian-one/au/pkg/au"
)

var Command = &cobra.Command{
	Use: "dev",
}

var dumpCommand = &cobra.Command{
	Use:  "export-workspace",
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

		dws, ok := ws.(au.DocProvider)
		if !ok {
			return errors.New("no access to doc")
		}
		doc := dws.GetDoc()

		encoder := yaml.NewEncoder(os.Stdout)
		encoder.SetIndent(2)
		return encoder.Encode(toTree(doc.Root()))
	},
}

var historyCommand = &cobra.Command{
	Use:  "show-workspace-history",
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

		dws, ok := ws.(au.DocProvider)
		if !ok {
			return errors.New("no access to doc")
		}
		doc := dws.GetDoc()

		output := make([]map[string]interface{}, 0)
		changes, err := doc.Changes()
		if err != nil {
			return errors.Wrap(err, "failed to get changes")
		}
		for _, change := range changes {
			dependencies := make([]string, 0)
			for _, hash := range change.Dependencies() {
				dependencies = append(dependencies, hash.String())
			}
			output = append(output, map[string]interface{}{
				"hash":         change.Hash().String(),
				"at":           change.Timestamp(),
				"message":      change.Message(),
				"dependencies": dependencies,
			})
		}
		encoder := yaml.NewEncoder(os.Stdout)
		encoder.SetIndent(2)
		return encoder.Encode(output)
	},
}

var generateDotCommand = &cobra.Command{
	Use:  "generate-dot",
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

		dws, ok := ws.(au.DocProvider)
		if !ok {
			return errors.New("no access to doc")
		}
		doc := dws.GetDoc()

		_, _ = fmt.Fprintln(os.Stdout, "strict digraph {")
		_, _ = fmt.Fprintf(os.Stdout, "node [colorscheme=pastel19]")
		changes, err := doc.Changes()
		if err != nil {
			return errors.Wrap(err, "failed to get changes")
		}
		for _, change := range changes {
			var color int
			{
				h := crc32.NewIEEE()
				_, _ = h.Write([]byte(change.ActorID()))
				color = 1 + int(h.Sum32()%9)
			}
			_, _ = fmt.Fprintf(
				os.Stdout, "\"%s\" [label=\"%s %s: '%s'\", style=\"filled\" fillcolor=%d]\n",
				change.Hash().String(), change.Hash().String()[:8], change.Timestamp().Format(time.RFC3339), change.Message(), color,
			)
			for _, hash := range change.Dependencies() {
				_, _ = fmt.Fprintf(os.Stdout, "\"%s\" -> \"%s\"\n", hash.String(), change.Hash().String())
			}
		}
		_, _ = fmt.Fprintln(os.Stdout, "}")
		return nil
	},
}

func toTree(item *automerge.Value) interface{} {
	switch item.Kind() {
	case automerge.KindMap:
		out := make(map[string]interface{}, item.Map().Len())
		keys, _ := item.Map().Keys()
		for _, k := range keys {
			x, _ := item.Map().Get(k)
			out[k] = toTree(x)
		}
		return out
	case automerge.KindList:
		out := make([]interface{}, item.List().Len())
		for i := range out {
			x, _ := item.List().Get(i)
			out[i] = toTree(x)
		}
		return out
	case automerge.KindStr:
		return item.Str()
	case automerge.KindBytes:
		return base64.StdEncoding.EncodeToString(item.Bytes())
	case automerge.KindText:
		return item.Text().String()
	case automerge.KindInt64:
		return item.Int64()
	case automerge.KindFloat64:
		return item.Float64()
	case automerge.KindBool:
		return item.Bool()
	case automerge.KindCounter:
		v, _ := item.Counter().Get()
		return v
	case automerge.KindNull:
		return nil
	case automerge.KindTime:
		return item.Time().Format(time.RFC3339)
	case automerge.KindUint64:
		return item.Uint64()
	default:
		return item.GoString()
	}
}

func init() {
	Command.AddCommand(
		dumpCommand,
		historyCommand,
		generateDotCommand,
	)
}
