package cmd

import (
	"github.com/go-sphere/sphere-cli/internal/entity"
	"github.com/go-sphere/sphere-cli/internal/entity/graph"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type EntSharedOptions struct {
	SchemaPath string

	AllFieldsRequired bool
	AutoAddAnnotation bool
	EnumUseRawType    bool
	SkipUnsupported   bool

	TimeProtoType        string
	UUIDProtoType        string
	UnsupportedProtoType string

	ProtoPackages string
}

func (o *EntSharedOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.SchemaPath, "path", "./schema", "path to schema directory")

	fs.StringVar(&o.TimeProtoType, "time_proto_type", "int64", "use proto type for time.Time, one of int64, string, google.protobuf.Timestamp")
	fs.StringVar(&o.UUIDProtoType, "uuid_proto_type", "string", "use proto type for uuid.UUID, one of string, bytes")
	fs.StringVar(&o.UnsupportedProtoType, "unsupported_proto_type", "google.protobuf.Any", "use proto type for unsupported types, one of google.protobuf.Any, google.protobuf.Struct, bytes")

	fs.BoolVar(&o.AllFieldsRequired, "all_fields_required", true, "ignore optional, use zero value instead")
	fs.BoolVar(&o.AutoAddAnnotation, "auto_annotation", true, "auto add annotation to the schema")
	fs.BoolVar(&o.EnumUseRawType, "enum_raw_type", true, "use string for enum")
	fs.BoolVar(&o.SkipUnsupported, "skip_unsupported", true, "skip unsupported types, when unsupportedProtoType is not set")

	fs.StringVar(&o.ProtoPackages, "import_proto", "google/protobuf/any.proto,google.protobuf,Any;", "import proto, format: path1,package1,type1,type2;path2,package2,type3,type4;")
}

func (o *EntSharedOptions) ToGraphOptions() *graph.Options {
	return &graph.Options{
		SchemaPath:           o.SchemaPath,
		AllFieldsRequired:    o.AllFieldsRequired,
		AutoAddAnnotation:    o.AutoAddAnnotation,
		EnumUseRawType:       o.EnumUseRawType,
		SkipUnsupported:      o.SkipUnsupported,
		TimeProtoType:        o.TimeProtoType,
		UUIDProtoType:        o.UUIDProtoType,
		UnsupportedProtoType: o.UnsupportedProtoType,
	}
}

var ent2protoCmd = &cobra.Command{
	Use:     "entproto",
	Aliases: []string{"ent2proto"},
	Short:   "Convert Ent schema to Protobuf definitions",
	Long:    `Convert Ent schema to Protobuf definitions, generating .proto files from Ent schema definitions.`,
}

var ent2mapperCmd = &cobra.Command{
	Use:     "entmapper",
	Aliases: []string{"ent2mapper"},
	Short:   "Convert Ent schema to Ent mapper",
	Long:    `Convert Ent schema to Ent mapper, generating mapper files from Ent schema definitions.`,
}

func init() {
	shareOptions := &EntSharedOptions{}
	shareOptions.AddFlags(ent2protoCmd.Flags())
	shareOptions.AddFlags(ent2mapperCmd.Flags())
	rootCmd.AddCommand(ent2protoCmd)
	rootCmd.AddCommand(ent2mapperCmd)

	{
		flag := ent2protoCmd.Flags()
		protoDir := flag.String("proto", "./proto", "path to proto directory")
		ent2protoCmd.RunE = func(cmd *cobra.Command, args []string) error {
			return entity.GenerateProto(&entity.ProtoOptions{
				Graph:    shareOptions.ToGraphOptions(),
				ProtoDir: *protoDir,
			})
		}

	}
	{
		flag := ent2mapperCmd.Flags()
		mapperDir := flag.String("mapper", "./mapper", "path to mapper directory")
		mapperPackage := flag.String("mapper_package", "mapper", "package name for the generated mapper code")
		entPackage := flag.String("ent_package", "ent", "package name for the ent code")
		protoPkgPath := flag.String("proto_pkg_path", "github.com/go-sphere/sphere-layout/proto", "go module path for the generated proto code")
		protoPkgName := flag.String("proto_pkg_name", "proto", "package name for the generated proto code")
		ent2mapperCmd.RunE = func(cmd *cobra.Command, args []string) error {
			return entity.GenerateMapper(&entity.MapperOptions{
				Graph:         shareOptions.ToGraphOptions(),
				MapperDir:     *mapperDir,
				MapperPackage: *mapperPackage,
				EntPackage:    *entPackage,
				ProtoPkgPath:  *protoPkgPath,
				ProtoPkgName:  *protoPkgName,
			})
		}
	}
}
