package cmd

import (
	"github.com/go-sphere/sphere-cli/internal/tags"
	"github.com/spf13/cobra"
)

var retagsCmd = &cobra.Command{
	Use:        "retags",
	Deprecated: "retags is deprecated, please use protoc-gen-sphere-binding directly.",
	Short:      "Inject custom tags to protobuf golang struct",
	Long:       `Refer to "favadi/protoc-go-inject-tag", which is specifically optimized for the sphere project.`,
}

func init() {
	rootCmd.AddCommand(retagsCmd)

	flag := retagsCmd.Flags()
	input := flag.String("input", "./api/*/*/*.pb.go", "pattern to match input file(s)")
	remove := flag.Bool("remove_tag_comment", true, "remove tag comment")
	omitJSON := flag.Bool("auto_omit_json", true, "omit JSON tags when form or uri struct tag is present")

	retagsCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return tags.ReTags(*input, *remove, *omitJSON)
	}
}
