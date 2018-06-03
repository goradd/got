// GoT (short for go templates) is a template engine that generates fast go templates that you compile into your go code.
//
// GoT is similar to some other template engines, like hero, in that it generates go code that can get compiled into your program,
// or a go plugin. This approach creates extremely fast templates, especially as
// compared to go's standard template engine. It also gives you much more freedom than Go's template
// engine, since at any time you can just switch to go code to do what you want.
//
//
// GoT's primary function is to support the goradd web framework, and that drives our
// design decisions. However, it is a standalone template engine that you may find useful.
//
// Features
//
// High performance. The templates utilize write buffers, which are known to be the fastest way to
// write strings in go. Since the resulting template is go code, your template will be compiled to fast
// machine code.
//
//
// Easy to use. The templates themselves are embedded into your go code. The template language is prettysimple and you can do a lot with only a few tags. You can switch into and out of go code at will. Tags are
// Mustache-like, so somewhat go idiomatic.
//
//
// Flexible. The template language makes very few assumptions about the go environment it is in. Most
// template engines require you to call the template with a specific function signature.
// GoT gives you the
// freedom to call your templates how you want.
//
//
// Translation Support. You can specify that you want to send your strings to a translator beforeoutput.
//
//
// Error Support. You can call into go code that returns errors, and have the template stop at that
// point and return an error to your wrapper function. The template will output its text up to that point,
// allowing you to easily see where in the template the error occurred.
//
//
// Include Files. Templates can include other templates. You can specify
// a list of search directories for the include files, allowing you to put include files in a variety of
// locations, and have include files in one directory that override another directory.
//
//
// Custom Tags. You can define named fragments that you can include at will,and you can define these fragments in include files. To use these fragments, you just
// use the name of the fragment as the tag. This essentially gives you the ability to create your own template
// language. When you use a custom tag, you can also include parameters that will
// replace placeholders in your fragment, giving you even more power to your custom tags.
//
//
// Using other go libraries, you can have your templates compile when they are changed,use buffer pools to increase performance, write to io.Writers, and more. Since the
// templates become go code, you can do what you imagine.
//
// Installation
//
// To install:
//
//   go get github.com/spekary/got/got
//
// GoT will format the resulting code using `go fmt`, but we recommend installing `goimports`
// and passing it the -i flag on the command line to use goimports instead, since that will add the
// additional service of fixing up the imports line.
//
//   go get golang.org/x/tools/cmd/goimports
//
// Usage
//
// From the command line:
//
//   got [options] [files]
//
//   options:
//   	- o: The output directory. If not specified, files will be output at the same location as the corresponding template.
//   	- t fileType: If set, will process all files in the current directory with this suffix. If not set, you must specify the files at the end of the command line.
//   	- i: Run `goimports` on the output files, rather than `go fmt`
//   	- I directories and/or files:  A list of semicolon separated directories and/or files. If a directory, it is used as 
//   		the search path for include files. If a file, it is automatically added to the front of every file that is
//   		processed.  Directories are searched in the order specified and first matching file will be used. It
//   		 will always look in the current directory last unless the current directory is specified
//   		 in the list in another location.
//
//   example:
//   	got -t got -i -o /templates
//   	got -I .;../tmpl;./projectTemplates file1.tmpl file2.tmpl
//
// Basic Syntax
//
// Template tags start with {{ and end with }}.
//
// A template starts in go mode. To send simple text or html to output, surround the text with {{ and }} tags
// with a space or newline separating the tags from the surrounding text. Inside the brackets you will be in
// text mode.
// From within text mode, you can send out a go value by surrounding the go code with {{ and }} tags without spaces
// separating the go code from the brackets.
//
//
// Text will get written to output by calling buf.WriteString. Got makes no assumptions
// as to how you declare the
// buf variable, it just needs to be available when the template text is declared.
// Usually you would do this by declaring a function at the top of your template that receives a
// buf *bytes.Buffer parameter. After compiling the template output together with your program, you call
// this function to get the template output.
//
//
// At a minimum, you will need to import the "bytes" package into the file with your template function.
// Depending on what tags you use, you might need to add
// additional items to your import list. Those are mentioned below with each tag.
//
//
// Example
//
// Here is how you might create a very basic template. For purposes of this example, we will call the file
// example.got and put it in the template package, but you can name the file and package whatever you want.
//
//   package template
//
//   import "bytes"
//
//   func OutTemplate(buf *bytes.Buffer) {
//   	var world string = "World"
//   {{
//   <p>
//       Hello {{world}}!
//   </p>
//   }}
//   }
//
// To compile this template, call got:
//
//   got example.got
//
// This will create an example.go file, which you include in your program. You then can call the function
// you declared:
//
//
//   package main
//
//   import (
//   	"bytes"
//   	"os"
//   	"template"
//   )
//
//   func main() {
//   	var b bytes.Buffer 
//   	template.OutTemplate(b)
//   	b.WriteTo(os.Stdout)
//   }
//
// This simple example shows a mix of go code and template syntax in the same file. Using GoT's include files,
// you can separate your go code from template code if you want.
//
//
// Template Syntax
//
// The following describes how open tags work. Most tags end with a }}, unless otherwise indicated.
// Many tags have a short and a long form. Using the long form does not impact performance, its just there
// to help your templates have some human readable context to them if you want that.
//
//
// Static Text
//
// The following tags will send the surrounded text to buf.WriteString:
//  Tag                        Description
//
//  {{<space or newline>       Begin to output text as written.
//  {{! or {{esc               Html escape the text. Html reserved characters, like < or > are turned into html entities first.
//  {{h or {{html              Html escape and html format breaks and newlines
//  {{t or {{translate         Send the text to a translator
//
// The {{! tag requires you to import the standard html package. {{h requires both the html and strings packages.
//
// {{t will wrap the static text with a call to t.Translate(). Its up to you to define this object and make it available to the template. The
// translation will happen during runtime of your program. We hope that a future implementation of GoT could
// have an option to send these strings to an i18n file to make it easy to send these to a translation service.
//
// From within any static text context described above you can switch into go context by using the
//
//   {{g or {{go tag           Start go mode.
//
// Example
//
//   package test
//
//   import (
//      "bytes"
//      "fmt"
//   )
//
//   type Translater interface {
//      Translate(string, *bytes.Buffer)
//   }
//
//   func staticTest(buf *bytes.Buffer) {
//   {{ Here {{go buf.WriteString("is") }} some code wrapping text escaping to go. }}
//
//   {{
//   <p>
//   {{! Escaped html < }}
//   </p>
//   }}
//
//   {{h
//      This is text
//      that is both escaped and has
//      html paragraphs and breaks inserted.
//   }}
//
//   }
//
//   func translateTest(t Translater, buf *bytes.Buffer) {
//
//   {{t Translate me to some language }}
//
//   {{!t Translate & escape me > }}
//
//   }
//
// Dynamic Text
//
// The following tags are designed to surround go code that returns a go value.
// The value will be converted to a string and sent to the output. The go code could be a static value, or a function
// that returns a value.
//
//  Tag                              Description                                Example
//
//  {{=, {{s, or  {{string.          Send a go string to output                 {{= fmt.Sprintf("I am %s", sVar) }}
//  {{i or {{int                     Send an int to output                      {{ The value is: {{i iVar }} }}
//  {{u or {{uint                    Send an unsigned Integer                   {{ The value is: {{u uVar }} }}
//  {{f or {{float                   Send a floating point number               {{ The value is: {{f fVar }} }}
//  {{b or {{bool                    A boolean (will output "true" or "false")  {{ The value is: {{b bVar }} }}
//  {{w or {{bytes                   A byte slice                               {{ The value is: {{w byteSliceVar }} }}
//  {{v or {{stringer or             Send any value that implements             {{ The value is: {{objVar}} }}
//  {{goIdentifier}}                 the Stringer interface.
//
// The last tag can be slower than
// the other tags since it uses fmt.Sprintf internally, so if this is a heavily used template,
// avoid it. Usually you will not notice a speed difference though, and the {{goIdentifier}}
// type of tag can be very convenient, which is simply any go variable surrounded by mustaches with no spaces.
//
//
// Escaping Dynamic Text
//
// Some value types potentially could produce html reserved characters. These tags will html escape
// the output.
//
//  Tag                            Description
//
//  {{!=, {{!s or {{!string        HTML escape a go string
//  {{!w or {{!bytes               HTML escape a byte slice
//  {{!v or {{!stringer            HTML escape a Stringer
//  {{!h                           Escape a go string and and html format breaks and newlines
//
// These tags require you to import the "html" package. The {{!h tag also requires the "strings" package.
//
//
// Capturing Errors
//
// These tags will receive two results, the first a value to send to output, and the second an error
// type. If the error is not nil, processing will stop and the error will be returned by the template function. Therefore, these
// tags expect to be included in a function that returns an error. Any template text
// processed so far will still be sent to the output buffer.
//
//  Tag                           Description
//
//  {{=e, {{se, {{string,err      Output a go string, capturing an error
//  {{!=e, {{!se, {{!string,err   HTML escape a go string and capture an error
//  {{ie or {{int,err             Output a go int and capture an error
//  {{ue or {{uint,err            Output a go uint and capture an error
//  {{fe or {{float,err           Output a go float64 and capture an error
//  {{be or {{bool,err            Output a bool ("true" or "false") and capture an error
//  {{we, {{bytes,err             Output a byte slice and capture an error
//  {{!we or {{!bytes,err         HTML escape a byte slice and capture an error
//  {{ve, {{stringer,err          Output a Stringer and capture an error
//  {{!ve or {{!stringer,err      HTML escape a Stringer and capture an error
//  {{e, or {{err                 Execute go code that returns an error, and stop if the error is not nil
//
// Example
//
//   func Tester(s string) (s string, err error) {
//      if s == "bad" {
//          err = errors.New("This is bad.")
//      }
//      return
//   }
//
//   func OutTemplate(toPrint string, buf bytes.Buffer) error {
//       {{=e Tester(toPrint) )}
//   }
//
// Include Files
//
//   {{: "fileName" }} or {{include "fileName" }}       Inserts the given file name into the template.
//
// The included file will start in whatever mode the receiving template is in, as if the text was inserted
// at that spot, so if the include tags are
// put inside of go code, the included file will start in go mode.
// The file will then be processed like any other got file.
//
// Include files are searched for in the current directory, and in the list of include directories provided
// on the command line by the -I option.
//
//
// Defined Fragments
//
// Defined fragments start a block of text that can be included later in a template. The included text will
// be sent as is, and then processed in whatever mode the template processor is in, as if that text was simply
// inserted into the template at that spot. You can include the {{ or {{g tags inside of the fragment to
// force the processor into the text or go modes if needed. The fragment can be defined
// any time before it is included, including being defined in other include files. You can add optional parameters
// to a fragment that will be substituted for placeholders when the fragment is used. You can have up to 9
// placeholders ($1 - $9). Parameters should be separated by commas, and can be surrounded by quotes if needed.
//
//  {{< fragName }} or {{define fragName }}          Start a block called "fragName".
//  {{> fragName param1,param2,...}} or              Substitute this tag for the given defined fragment.
//  {{put fragName param1,param2,...}} or just
//  {{fragName param1,param2,...}}
//
// If a defined fragment is not defined with the given name, got will panic and stop compiling.
// param1, param2, ... are optional parameters that will be substituted for $1, $2, ... in the defined fragment.
//
// The fragment name is NOT surrounded by quotes, and cannot contain any whitespace in the name. Blocks are ended with a
// {{end}} tag. The end tag must be just like that, with no spaces inside the tag.
//
// Example
//
//   {{< hFrag }}
//   <p>
//   This is my html body.
//   </p>
//   {{end}}
//
//   {{< writeMe }}
//   {{// The g tag here forces us to process the text as go code, no matter where the fragment is included }}
//   {{g 
//   if $2 {
//     buf.WriteString("$1")
//   }
//   }}
//   {{end}}
//
//
//   func OutTemplate(buf bytes.Buffer) {
//   {{
//      <html>
//        <body>
//          {{> hFrag }}
//        </body>
//      </html>
//   }}
//
//   {{writeMe "Help Me!", true}}
//   }
//
// Other Tags
//
//  {{g or {{go                           Switch into go mode from within a static text mode
//  {{# or {{//                           Comment the template. This is removed from the compiled template.
//  {{- }} or {{backup }}                 Backs up one characters.
//  {{- <count>}} or {{backup <count>}}   Back up <count> characters.
//  {{if <go condition>}}<block>{{if}}    This is a convenience tag for surrounding text with a go "if" statement.
//  {{for <go condition>}}<block>{{for}}  This is a convenience tag for surrounding text with a go "for" statement.
//
//
// You can put a {{else}} in between to have an if/else. These kinds of if/else statements
// are easier to read than putting them inside of {{g tags.
//
// Examples
//
//  {{if plural }}We are{{else}}I am{{if}} awesome.
//  {{for num,item := range items }}<p>Item {{num}} is {{item}}</p>{{for}}
//
//
// Bigger Example
//
// In this example, we will combine multiple files. One, a traditional html template with a place to fill in
// some body text. Another, a go function declaration that we will use when we want to draw the template.
// The function will use a traditional web server io.Writer pattern, including the use of a context parameter.
// Finally, there is an example main.go illustrating how our template function would be called.
//
//
// index.html
//
//   <!DOCTYPE html>
//   <html>
//       <head>
//           <meta charset="utf-8">
//       </head>
//
//       <body>
//   {{# The tag below declares that we want to substitute a named fragment that is declared elsewhere }}
//   {{put body }}
//       </body>
//   </html>
//
// template.got
//
//   package main
//
//   import {
//      "context"
//      "bytes"
//   }
//
//
//
//   func writeTemplate(ctx context.Context, buf *bytes.Buffer) {
//
//   {{# Define the body that will be inserted into the template }}
//   {{< body }}
//   <p>
//   The caller is: {{=s ctx.Value("something") }}
//   </p>
//   {{end}}
//
//   {{# include the html template. Since the template is html, we need to put ourselves in static text mode first }}
//   {{ 
//   {{include "index.html"}}
//   }}
//
//   }
//
// main.go
//
//   type myHandler struct {}
//
//   func (h myHandler) ServeHTTP(w http.ResponseWriter, r *http.Request)  {
//      ctx :=  context.WithValue(r.Context(), "caller", r.Referer())
//      b := new(bytes.Buffer)
//      writeTemplate(ctx, b)	// call the got template
//      w.Write(b.Bytes())
//   }
//
//
//   func main() {
//      var r myHandler
//      var err error
//
//      *local = "0.0.0.0:8000"
//
//      err = http.ListenAndServe(*local, r)
//
//      if err != nil {
//         fmt.Println(err)
//      }
//   }
//
// To compile the template:
//
//   got template.got
//
// Build your application and go to http://localhost:8000 in your browser, to see your results
//
// License
//
// Got is licensed under the MIT License.
//
//
package main
