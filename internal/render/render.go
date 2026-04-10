package render

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
)

type Renderer struct {
	out      io.Writer
	jsonMode bool
	mdRender *glamour.TermRenderer
}

func New(jsonMode bool) *Renderer {
	r, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)
	return &Renderer{
		out:      os.Stdout,
		jsonMode: jsonMode,
		mdRender: r,
	}
}

func (r *Renderer) IsJSON() bool {
	return r.jsonMode
}

func (r *Renderer) JSON(v any) {
	enc := json.NewEncoder(r.out)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

func (r *Renderer) Title(s string) {
	fmt.Fprintln(r.out)
	fmt.Fprintln(r.out, TitleStyle.Render(s))
}

func (r *Renderer) Header(s string) {
	fmt.Fprintln(r.out, HeaderStyle.Render(s))
}

func (r *Renderer) Body(s string) {
	fmt.Fprintln(r.out, BodyStyle.Render(s))
}

func (r *Renderer) URL(s string) {
	fmt.Fprintln(r.out, URLStyle.Render(s))
}

func (r *Renderer) Meta(s string) {
	fmt.Fprintln(r.out, MetaStyle.Render(s))
}

func (r *Renderer) Error(s string) {
	fmt.Fprintln(r.out, ErrorStyle.Render("I regret to inform you, sir: "+s))
}

func (r *Renderer) Warn(s string) {
	fmt.Fprintln(r.out, MetaStyle.Render("A word of caution, sir: "+s))
}

func (r *Renderer) Success(s string) {
	fmt.Fprintln(r.out, SuccessStyle.Render(s))
}

func (r *Renderer) Markdown(md string) {
	if r.mdRender != nil {
		rendered, err := r.mdRender.Render(md)
		if err == nil {
			fmt.Fprint(r.out, rendered)
			return
		}
	}
	fmt.Fprintln(r.out, md)
}

func (r *Renderer) Divider() {
	fmt.Fprintln(r.out, Divider())
}

func (r *Renderer) Score(label string, score float64) {
	bar := strings.Repeat("█", int(score*20))
	pad := strings.Repeat("░", 20-int(score*20))
	fmt.Fprintf(r.out, "  %s %s%s %.1f\n",
		MetaStyle.Render(label),
		ScoreStyle.Render(bar),
		DividerStyle.Render(pad),
		score,
	)
}

func (r *Renderer) Welcome(dir string) {
	fmt.Fprintln(r.out)
	fmt.Fprintln(r.out, TitleStyle.Render("Good evening, sir."))
	fmt.Fprintln(r.out)
	fmt.Fprintf(r.out, "%s\n", BodyStyle.Render(
		fmt.Sprintf("I've prepared your research library at %s.", dir)))
	fmt.Fprintf(r.out, "%s\n", BodyStyle.Render(
		"You'll want to configure a search provider:"))
	fmt.Fprintln(r.out)
	fmt.Fprintf(r.out, "  %s\n", HeaderStyle.Render(
		"jeeves config set search.brave_api_key <key>"))
	fmt.Fprintln(r.out)
	fmt.Fprintf(r.out, "%s\n", MetaStyle.Render(
		"Glance resolution works with just a search API key — zero LLM setup required."))
	fmt.Fprintln(r.out)
}
