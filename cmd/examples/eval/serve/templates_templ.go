// Code generated by templ - DO NOT EDIT.

// templ: version: v0.3.833
package serve

//lint:file-ignore SA4006 This context is only used if a nested component is present.

import "github.com/a-h/templ"
import templruntime "github.com/a-h/templ/runtime"

func page(outputs []RenderableTestOutput) templ.Component {
	return templruntime.GeneratedTemplate(func(templ_7745c5c3_Input templruntime.GeneratedComponentInput) (templ_7745c5c3_Err error) {
		templ_7745c5c3_W, ctx := templ_7745c5c3_Input.Writer, templ_7745c5c3_Input.Context
		if templ_7745c5c3_CtxErr := ctx.Err(); templ_7745c5c3_CtxErr != nil {
			return templ_7745c5c3_CtxErr
		}
		templ_7745c5c3_Buffer, templ_7745c5c3_IsBuffer := templruntime.GetBuffer(templ_7745c5c3_W)
		if !templ_7745c5c3_IsBuffer {
			defer func() {
				templ_7745c5c3_BufErr := templruntime.ReleaseBuffer(templ_7745c5c3_Buffer)
				if templ_7745c5c3_Err == nil {
					templ_7745c5c3_Err = templ_7745c5c3_BufErr
				}
			}()
		}
		ctx = templ.InitializeContext(ctx)
		templ_7745c5c3_Var1 := templ.GetChildren(ctx)
		if templ_7745c5c3_Var1 == nil {
			templ_7745c5c3_Var1 = templ.NopComponent
		}
		ctx = templ.ClearChildren(ctx)
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 1, "<!doctype html><html><head><title>Story Generation Results</title><link href=\"https://cdn.jsdelivr.net/npm/bootstrap@5.1.3/dist/css/bootstrap.min.css\" rel=\"stylesheet\"><script src=\"https://unpkg.com/htmx.org@1.9.10\"></script><style>\n\t\t\t\t.story-card { margin-bottom: 2rem; }\n\t\t\t\t.keywords { color: #666; }\n\t\t\t\t.story-text { font-size: 1.1rem; line-height: 1.6; }\n\t\t\t</style></head><body><div class=\"container mt-4\"><h1 class=\"mb-4\">Story Generation Results</h1><div class=\"row\">")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		for _, output := range outputs {
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 2, "<div class=\"col-12 story-card\"><div class=\"card\"><div class=\"card-body\"><h5 class=\"card-title\">Story #")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var2 string
			templ_7745c5c3_Var2, templ_7745c5c3_Err = templ.JoinStringErrs(output.EntryID)
			if templ_7745c5c3_Err != nil {
				return templ.Error{Err: templ_7745c5c3_Err, FileName: `cmd/examples/eval/serve/templates.templ`, Line: 24, Col: 54}
			}
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var2))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 3, "</h5><div class=\"mb-3\"><strong>Topic:</strong> ")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var3 string
			templ_7745c5c3_Var3, templ_7745c5c3_Err = templ.JoinStringErrs(output.Topic)
			if templ_7745c5c3_Err != nil {
				return templ.Error{Err: templ_7745c5c3_Err, FileName: `cmd/examples/eval/serve/templates.templ`, Line: 26, Col: 47}
			}
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var3))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 4, "<br><strong>Age:</strong> ")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var4 string
			templ_7745c5c3_Var4, templ_7745c5c3_Err = templ.JoinStringErrs(output.Age)
			if templ_7745c5c3_Err != nil {
				return templ.Error{Err: templ_7745c5c3_Err, FileName: `cmd/examples/eval/serve/templates.templ`, Line: 27, Col: 43}
			}
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var4))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 5, "<br><strong>Moral:</strong> ")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var5 string
			templ_7745c5c3_Var5, templ_7745c5c3_Err = templ.JoinStringErrs(output.Moral)
			if templ_7745c5c3_Err != nil {
				return templ.Error{Err: templ_7745c5c3_Err, FileName: `cmd/examples/eval/serve/templates.templ`, Line: 28, Col: 47}
			}
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var5))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 6, "</div><div class=\"story-text mb-3\">")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var6 string
			templ_7745c5c3_Var6, templ_7745c5c3_Err = templ.JoinStringErrs(output.LastMessage)
			if templ_7745c5c3_Err != nil {
				return templ.Error{Err: templ_7745c5c3_Err, FileName: `cmd/examples/eval/serve/templates.templ`, Line: 31, Col: 29}
			}
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var6))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 7, "</div><div class=\"keywords mb-3\"><strong>Expected Keywords:</strong> ")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			for _, keyword := range output.GoldenAnswer {
				templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 8, "<span class=\"badge bg-secondary me-1\">")
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				var templ_7745c5c3_Var7 string
				templ_7745c5c3_Var7, templ_7745c5c3_Err = templ.JoinStringErrs(keyword)
				if templ_7745c5c3_Err != nil {
					return templ.Error{Err: templ_7745c5c3_Err, FileName: `cmd/examples/eval/serve/templates.templ`, Line: 36, Col: 57}
				}
				_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var7))
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 9, "</span>")
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 10, "</div>")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templ.RenderScriptItems(ctx, templ_7745c5c3_Buffer, toggleConversation(output.EntryID))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 11, "<button class=\"btn btn-sm btn-primary\" hx-get=\"#\" hx-target=\"")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var8 string
			templ_7745c5c3_Var8, templ_7745c5c3_Err = templ.JoinStringErrs("#conversation-" + output.EntryID)
			if templ_7745c5c3_Err != nil {
				return templ.Error{Err: templ_7745c5c3_Err, FileName: `cmd/examples/eval/serve/templates.templ`, Line: 42, Col: 54}
			}
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var8))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 12, "\" onclick=\"")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var9 templ.ComponentScript = toggleConversation(output.EntryID)
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ_7745c5c3_Var9.Call)
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 13, "\">Show Full Conversation</button><div id=\"")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var10 string
			templ_7745c5c3_Var10, templ_7745c5c3_Err = templ.JoinStringErrs("conversation-" + output.EntryID)
			if templ_7745c5c3_Err != nil {
				return templ.Error{Err: templ_7745c5c3_Err, FileName: `cmd/examples/eval/serve/templates.templ`, Line: 47, Col: 50}
			}
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var10))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 14, "\" class=\"mt-3\" style=\"display: none;\"><pre class=\"bg-light p-3\"><code>")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var11 string
			templ_7745c5c3_Var11, templ_7745c5c3_Err = templ.JoinStringErrs(output.ConversationString)
			if templ_7745c5c3_Err != nil {
				return templ.Error{Err: templ_7745c5c3_Err, FileName: `cmd/examples/eval/serve/templates.templ`, Line: 48, Col: 68}
			}
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var11))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 15, "</code></pre></div></div></div></div>")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 16, "</div></div><script>\n\t\t\t\tfunction toggleConversation(id) {\n\t\t\t\t\tconst conv = document.getElementById('conversation-' + id);\n\t\t\t\t\tif (conv.style.display === 'none') {\n\t\t\t\t\t\tconv.style.display = 'block';\n\t\t\t\t\t} else {\n\t\t\t\t\t\tconv.style.display = 'none';\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t</script></body></html>")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		return nil
	})
}

func toggleConversation(id string) templ.ComponentScript {
	return templ.ComponentScript{
		Name: `__templ_toggleConversation_9a50`,
		Function: `function __templ_toggleConversation_9a50(id){toggleConversation(id)
}`,
		Call:       templ.SafeScript(`__templ_toggleConversation_9a50`, id),
		CallInline: templ.SafeScriptInline(`__templ_toggleConversation_9a50`, id),
	}
}

var _ = templruntime.GeneratedTemplate
