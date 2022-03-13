package core

import (
	"os"
	"reflect"
	"strings"

	"github.com/thought-machine/go-flags"
)

// AttachAliasFlags attaches the alias flags to the given flag parser.
// It returns true if any modifications were made.
func (config *Configuration) AttachAliasFlags(parser *flags.Parser) bool {
	for name, alias := range config.Alias {
		cmd := parser.Command
		fields := strings.Fields(name)
		positionalLabels := alias.PositionalLabels || alias.RequiredLabel != ""
		for i, namePart := range fields {
			cmd = addSubcommand(cmd, namePart, alias.Desc, positionalLabels && len(alias.Subcommand) == 0 && i == len(fields)-1, alias.RequiredLabel)
			for _, subcommand := range alias.Subcommand {
				addSubcommands(cmd, strings.Fields(subcommand), positionalLabels, alias.RequiredLabel)
			}
			for _, flag := range alias.Flag {
				var f struct {
					Data bool
				}
				cmd.AddOption(&flags.Option{
					LongName: strings.TrimLeft(flag, "-"),
				}, &f.Data)
			}
		}
	}
	return len(config.Alias) > 0
}

// addSubcommands attaches a series of subcommands to the given command.
func addSubcommands(cmd *flags.Command, subcommands []string, positionalLabels bool, requiredLabel string) {
	if len(subcommands) > 0 && cmd != nil {
		addSubcommands(addSubcommand(cmd, subcommands[0], "", positionalLabels, requiredLabel), subcommands[1:], positionalLabels, requiredLabel)
	}
}

// addSubcommand adds a single subcommand to the given command.
// If one by that name already exists, it is returned.
func addSubcommand(cmd *flags.Command, subcommand, desc string, positionalLabels bool, requiredLabel string) *flags.Command {
	if existing := cmd.Find(subcommand); existing != nil {
		return existing
	}
	var data interface{} = &struct{}{}
	if positionalLabels {
		label := reflect.StructOf([]reflect.StructField{{
			Name: "Label",
			Type: reflect.TypeOf(aliasLabel{}),
			Tag:  reflect.StructTag(`required-label:"` + requiredLabel + `"`),
		}})
		if _, ok := reflect.New(label).Interface().(flags.Completer); !ok {
			log.Fatalf("Failed to create a valid Completer")
		}
		args := reflect.StructOf([]reflect.StructField{{
			Name: "Target",
			Type: reflect.SliceOf(label),
			Tag:  reflect.StructTag(`positional-arg-name:"target" description:"Build targets"`),
		}})
		d := reflect.StructOf([]reflect.StructField{{
			Name: "Args",
			Type: args,
			Tag:  reflect.StructTag(`positional-args:"true"`),
		}})
		data = reflect.New(d).Interface()
	}
	newCmd, _ := cmd.AddCommand(subcommand, desc, desc, data)
	return newCmd
}

// An aliasLabel is used for completing build labels on aliases. Some fiddling is needed
// since completion is a property of the type, rather than of the value.
type aliasLabel struct {
	Label         BuildLabel
	RequiredLabel string
}

func (label aliasLabel) Complete(match string) []flags.Completion {
	os.Setenv("PLZ_COMPLETE_LABEL", label.RequiredLabel)
	return label.Label.Complete(match)
}
