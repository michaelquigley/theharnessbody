// Package command is a small, transport-agnostic command dispatcher. Commands are
// registered by name with a handler; Dispatch parses a line of text — a chat
// message with its trigger word or @mentions already stripped, or a CLI argument
// string — and routes it to the handler, returning the text to reply with. The
// same registry can serve a chat bot and a CLI, so handlers are written once.
package command

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Handler runs one command. args are the whitespace-separated tokens after the
// command name. The returned string is the reply; an error is rendered into the
// reply by Dispatch.
type Handler func(ctx context.Context, args []string) (string, error)

type entry struct {
	summary string
	handler Handler
}

// Registry holds the registered commands.
type Registry struct {
	commands map[string]entry
}

// New returns an empty registry.
func New() *Registry {
	return &Registry{commands: make(map[string]entry)}
}

// Register adds a command, matched case-insensitively. summary is shown in Help.
// Registering the same name twice replaces the earlier handler. Register panics on
// misuse — a blank name, the reserved name "help", or a nil handler — since these
// are setup-time programmer errors that would otherwise yield a command that can
// never be dispatched or that panics when it is.
func (r *Registry) Register(name string, summary string, handler Handler) {
	name = strings.ToLower(strings.TrimSpace(name))
	switch {
	case name == "":
		panic("command: Register called with a blank name")
	case name == "help":
		panic(`command: "help" is reserved`)
	case handler == nil:
		panic("command: Register called with a nil handler for command " + name)
	}
	r.commands[name] = entry{summary: summary, handler: handler}
}

// Dispatch parses commandText (a command name plus args, already stripped of any
// chat trigger word or mentions) and routes it. It always returns a reply: help
// for an empty line or "help", the handler's output (or a rendered error) for a
// known command, and an unknown-command message plus help otherwise.
func (r *Registry) Dispatch(ctx context.Context, commandText string) string {
	tokens := strings.Fields(strings.TrimSpace(commandText))
	if len(tokens) == 0 {
		return r.Help()
	}

	name := strings.ToLower(tokens[0])
	args := tokens[1:]

	if name == "help" {
		return r.Help()
	}

	e, ok := r.commands[name]
	if !ok {
		return fmt.Sprintf("unknown command '%s'\n\n%s", name, r.Help())
	}

	reply, err := e.handler(ctx, args)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return reply
}

// Help lists the registered commands and their summaries.
func (r *Registry) Help() string {
	names := make([]string, 0, len(r.commands))
	for name := range r.commands {
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	b.WriteString("available commands:")
	for _, name := range names {
		b.WriteString("\n  ")
		b.WriteString(name)
		if s := r.commands[name].summary; s != "" {
			b.WriteString(" - ")
			b.WriteString(s)
		}
	}
	b.WriteString("\n  help - show this message")
	return b.String()
}
