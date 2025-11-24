package entity

import (
	"errors"
	"fmt"
	"go/format"
	"log"
	"os"
	"path"
	"path/filepath"

	"entgo.io/contrib/entproto"
	"entgo.io/ent/entc/gen"
	"github.com/go-sphere/sphere-cli/internal/entity/graph"
	"github.com/go-sphere/sphere-cli/internal/entity/mapper"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoprint"
)

type ProtoOptions struct {
	Graph    *graph.Options
	ProtoDir string
}

func GenerateProto(options *ProtoOptions) error {
	gh, err := graph.LoadGraph(options.Graph)
	if err != nil {
		return err
	}
	err = generateProto(gh, options)
	if err != nil {
		return err
	}
	return nil
}

func generateProto(g *gen.Graph, options *ProtoOptions) error {
	entProtoDir := path.Join(g.Target, "proto")
	if options.ProtoDir != "" {
		entProtoDir = options.ProtoDir
	}
	adapter, err := entproto.LoadAdapter(g)
	if err != nil {
		return fmt.Errorf("entproto: failed parsing entity graph: %w", err)
	}
	var errs []error
	for _, schema := range g.Schemas {
		name := schema.Name
		_, sErr := adapter.GetFileDescriptor(name)
		if sErr != nil && !errors.Is(sErr, entproto.ErrSchemaSkipped) {
			errs = append(errs, sErr)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("entproto: failed parsing some schemas: %w", errors.Join(errs...))
	}
	allDescriptors := make([]*desc.FileDescriptor, 0, len(adapter.AllFileDescriptors()))
	for _, fDesc := range adapter.AllFileDescriptors() {
		graph.FixProto3Optional(g, fDesc)
		allDescriptors = append(allDescriptors, fDesc)
	}
	var printer protoprint.Printer
	printer.Compact = true
	if err = printer.PrintProtosToFileSystem(allDescriptors, entProtoDir); err != nil {
		return fmt.Errorf("entproto: failed writing .proto files: %w", err)
	}
	return nil
}

type MapperOptions struct {
	Graph         *graph.Options
	MapperDir     string
	MapperPackage string
	EntPackage    string
	ProtoPkgPath  string
	ProtoPkgName  string
}

func GenerateMapper(options *MapperOptions) error {
	gh, err := graph.LoadGraph(options.Graph)
	if err != nil {
		return err
	}
	err = generateMappers(gh, options)
	if err != nil {
		return err
	}
	return nil
}

func generateMappers(graph *gen.Graph, options *MapperOptions) error {
	adapter, err := entproto.LoadAdapter(graph)
	if err != nil {
		return fmt.Errorf("entproto: failed loading adapter: %w", err)
	}
	_ = os.RemoveAll(options.MapperDir)
	err = os.MkdirAll(options.MapperDir, 0755)
	if err != nil {
		return fmt.Errorf("entproto: failed creating entmapper dir: %w", err)
	}
	for _, node := range graph.Nodes {
		msgDesc, nErr := adapter.GetMessageDescriptor(node.Name)
		if nErr != nil {
			continue
		}
		fileDesc := msgDesc.GetFile()
		g, nErr := mapper.NewGenerator(fileDesc, graph, adapter, node, msgDesc)
		if nErr != nil {
			return nErr
		}

		if options.EntPackage != "" {
			g.EntPackage = mapper.GoImportPath(options.EntPackage)
		}
		if options.ProtoPkgPath != "" {
			g.ProtoImportPath = mapper.GoImportPath(options.ProtoPkgPath)
		}
		if options.MapperPackage != "" {
			g.PackageName = options.MapperPackage
		}
		if options.ProtoPkgName != "" {
			g.ProtoPackageName = options.ProtoPkgName
		}
		content, nErr := g.Generate()
		if nErr != nil {
			return nErr
		}
		formatted, fmtErr := format.Source(content)
		if fmtErr != nil {
			return fmt.Errorf("entproto: format entmapper for %s: %w", node.Name, fmtErr)
		}
		fileName := gen.Funcs["snake"].(func(string) string)(node.Name) + ".go"
		outPath := filepath.Join(options.MapperDir, fileName)
		log.Printf("entproto: generating entmapper for %s to %s", node.Name, outPath)
		nErr = os.WriteFile(outPath, formatted, 0644)
		if nErr != nil {
			return nErr
		}
	}
	return nil
}
