package devcmd

import (
	"encoding/base64"
	"os"
	"path/filepath"
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
	Use:  "export",
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
		encoder := yaml.NewEncoder(os.Stdout)
		encoder.SetIndent(2)
		return encoder.Encode(toTree(doc.Root()))
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
	)
}
