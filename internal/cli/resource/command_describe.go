package resource

import (
	"fmt"
	"io"
	"strings"

	debugctx "github.com/crmarques/declarest/debugctx"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/crmarques/declarest/metadata"
	"github.com/spf13/cobra"
)

func newDescribeCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	var pathFlag string

	command := &cobra.Command{
		Use:   "describe [path]",
		Short: "Describe resource structure and schema",
		Long: `Describe a resource or collection, showing its metadata, available operations,
and payload schema inferred from the OpenAPI definition. Useful for understanding
what fields are expected when creating or updating a resource.`,
		Example: strings.Join([]string{
			"  declarest resource describe /realms/master/clients/",
			"  declarest resource describe /realms/master/clients/my-client",
			"  declarest resource describe --path /jobs/ -o json",
		}, "\n"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := cliutil.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}

			debugctx.Printf(command.Context(), "resource describe requested path=%q", resolvedPath)

			outputFormat, err := cliutil.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			metadataService, err := cliutil.RequireMetadataService(deps)
			if err != nil {
				debugctx.Printf(command.Context(), "resource describe failed path=%q error=%v", resolvedPath, err)
				return err
			}

			md, err := resolveMetadataForDescribe(command, deps, metadataService, resolvedPath)
			if err != nil {
				debugctx.Printf(command.Context(), "resource describe failed path=%q error=%v", resolvedPath, err)
				return err
			}

			var openAPISpec any
			orchestratorService, err := cliutil.RequireOrchestrator(deps)
			if err == nil {
				openAPIContent, specErr := orchestratorService.GetOpenAPISpec(command.Context())
				if specErr == nil {
					openAPISpec = openAPIContent.Value
				}
			}

			description := metadata.DescribeResource(resolvedPath, md, openAPISpec)

			debugctx.Printf(command.Context(), "resource describe succeeded path=%q", resolvedPath)

			return cliutil.WriteOutput(command, outputFormat, description, renderDescribeText)
		},
	}

	cliutil.BindPathFlag(command, &pathFlag)
	cliutil.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = cliutil.SinglePathArgCompletionFunc(deps)
	return command
}

func resolveMetadataForDescribe(
	command *cobra.Command,
	deps cliutil.CommandDependencies,
	service metadata.MetadataService,
	logicalPath string,
) (metadata.ResourceMetadata, error) {
	md, err := service.ResolveForPath(command.Context(), logicalPath)
	if err == nil {
		return md, nil
	}
	if !faults.IsCategory(err, faults.NotFoundError) {
		return metadata.ResourceMetadata{}, err
	}

	// Fallback: try OpenAPI inference
	var openAPISpec any
	orchestratorService, orchErr := cliutil.RequireOrchestrator(deps)
	if orchErr == nil {
		openAPIContent, specErr := orchestratorService.GetOpenAPISpec(command.Context())
		if specErr == nil {
			openAPISpec = openAPIContent.Value
		}
	}

	if openAPISpec != nil {
		inferred, inferErr := metadata.InferFromOpenAPISpec(
			command.Context(),
			logicalPath,
			metadata.InferenceRequest{},
			openAPISpec,
		)
		if inferErr == nil {
			return inferred, nil
		}
	}

	return metadata.ResourceMetadata{}, err
}

func renderDescribeText(w io.Writer, desc metadata.ResourceDescription) error {
	fmt.Fprintf(w, "%s\n", desc.Path)

	if desc.Identity != nil {
		fmt.Fprintf(w, "\nIdentity\n")
		if desc.Identity.ID != "" {
			fmt.Fprintf(w, "  id:    %s\n", desc.Identity.ID)
		}
		if desc.Identity.Alias != "" {
			fmt.Fprintf(w, "  alias: %s\n", desc.Identity.Alias)
		}
	}

	if desc.PayloadType != "" || desc.CollectionPath != "" {
		fmt.Fprintf(w, "\nMetadata\n")
		if desc.PayloadType != "" {
			fmt.Fprintf(w, "  payload type: %s\n", desc.PayloadType)
		}
		if desc.CollectionPath != "" {
			fmt.Fprintf(w, "  collection:   %s\n", desc.CollectionPath)
		}
	}

	if len(desc.RequiredFields) > 0 {
		fmt.Fprintf(w, "  required:     %s\n", strings.Join(desc.RequiredFields, ", "))
	}
	if len(desc.SecretFields) > 0 {
		fmt.Fprintf(w, "  secrets:      %s\n", strings.Join(desc.SecretFields, ", "))
	}

	if len(desc.Operations) > 0 {
		fmt.Fprintf(w, "\nOperations\n")

		maxNameLen := 0
		maxMethodLen := 0
		for _, op := range desc.Operations {
			if len(op.Name) > maxNameLen {
				maxNameLen = len(op.Name)
			}
			if len(op.Method) > maxMethodLen {
				maxMethodLen = len(op.Method)
			}
		}

		for _, op := range desc.Operations {
			fmt.Fprintf(w, "  %-*s  %-*s  %s\n", maxNameLen, op.Name, maxMethodLen, op.Method, op.Path)
		}
	}

	for _, schema := range desc.Schemas {
		fmt.Fprintf(w, "\nSchema (%s %s %s)\n", schema.Operation, schema.Source, schema.Method+" "+schema.Path)
		renderSchemaNodes(w, schema.Properties, "  ")
	}

	return nil
}

func renderSchemaNodes(w io.Writer, nodes []metadata.SchemaNode, indent string) {
	if len(nodes) == 0 {
		return
	}

	maxNameLen := computeMaxNameLen(nodes)
	maxTypeLen := computeMaxTypeLen(nodes)

	for _, node := range nodes {
		name := indent + node.Name
		padded := name + strings.Repeat(" ", maxNameLen+len(indent)+2-len(name))
		typePadded := node.Type + strings.Repeat(" ", maxTypeLen+2-len(node.Type))

		annotations := buildAnnotations(node)
		if annotations != "" {
			fmt.Fprintf(w, "%s%s%s\n", padded, typePadded, annotations)
		} else {
			fmt.Fprintf(w, "%s%s\n", padded, strings.TrimRight(typePadded, " "))
		}

		if len(node.Properties) > 0 {
			renderSchemaNodes(w, node.Properties, indent+"  ")
		}
		if node.Items != nil && len(node.Items.Properties) > 0 {
			renderSchemaNodes(w, node.Items.Properties, indent+"  ")
		}
	}
}

func buildAnnotations(node metadata.SchemaNode) string {
	var parts []string

	if node.Required {
		parts = append(parts, "*required")
	}
	if node.Nullable {
		parts = append(parts, "nullable")
	}
	if node.Default != nil {
		parts = append(parts, fmt.Sprintf("default: %v", node.Default))
	}
	if len(node.Enum) > 0 {
		if len(node.Enum) <= 6 {
			parts = append(parts, "enum: ["+strings.Join(node.Enum, ", ")+"]")
		} else {
			parts = append(parts, fmt.Sprintf("enum: [%s, ... +%d more]",
				strings.Join(node.Enum[:4], ", "), len(node.Enum)-4))
		}
	}
	if node.Pattern != "" {
		parts = append(parts, "pattern: "+node.Pattern)
	}
	if node.MinLength != nil || node.MaxLength != nil {
		constraint := "length: "
		if node.MinLength != nil && node.MaxLength != nil {
			constraint += fmt.Sprintf("%d..%d", *node.MinLength, *node.MaxLength)
		} else if node.MinLength != nil {
			constraint += fmt.Sprintf(">=%d", *node.MinLength)
		} else {
			constraint += fmt.Sprintf("<=%d", *node.MaxLength)
		}
		parts = append(parts, constraint)
	}
	if node.Minimum != nil || node.Maximum != nil {
		constraint := "range: "
		if node.Minimum != nil && node.Maximum != nil {
			constraint += fmt.Sprintf("%g..%g", *node.Minimum, *node.Maximum)
		} else if node.Minimum != nil {
			constraint += fmt.Sprintf(">=%g", *node.Minimum)
		} else {
			constraint += fmt.Sprintf("<=%g", *node.Maximum)
		}
		parts = append(parts, constraint)
	}
	if node.Description != "" {
		desc := node.Description
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		parts = append(parts, desc)
	}

	return strings.Join(parts, "  ")
}

func computeMaxNameLen(nodes []metadata.SchemaNode) int {
	max := 0
	for _, node := range nodes {
		if len(node.Name) > max {
			max = len(node.Name)
		}
	}
	return max
}

func computeMaxTypeLen(nodes []metadata.SchemaNode) int {
	max := 0
	for _, node := range nodes {
		if len(node.Type) > max {
			max = len(node.Type)
		}
	}
	return max
}
