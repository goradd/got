https://img.shields.io/github/workflow/status/goradd/got/Go
# GoT

GoT (short for go templates) is a template engine that generates fast go templates. 

It is similar to some other 
template engines, like [hero](https://github.com/shiyanhui/hero), in that it generates go code that is then compiled 
into your program or a go plugin. This approach creates extremely fast templates, especially as
compared to go's standard template engine. It also gives you much more freedom than Go's template
engine, since at any time you can just switch to go code to do what you want.

- [Features](#features)
- [Install](#install)
- [Usage](#usage)
- [Quick Start](#quick-start)
- [Template Syntax](#template-syntax)
- [License](#license)

## Features

- **High performance**. The templates utilize write buffers, which are known to be the fastest way to
write strings in go. Since the resulting template is go code, your template will be compiled to fast
machine code.
- **Easy to use**. The templates themselves are embedded into your go code. The template language is pretty 
simple and you can do a lot with only a few tags. You can switch into and out of go code at will. Tags are 
Mustache-like, so somewhat go idiomatic.
- **Flexible**. The template language makes very few assumptions about the go environment it is in. Most other
template engines require you to call the template with a specific function signature. **GoT** gives you the
freedom to call your templates how you want.
- **Translation Support**. You can specify that you want to send your strings to a translator before 
output.
- **Error Support**. You can call into go code that returns errors, and have the template stop at that
point and return an error to your wrapper function. The template will output its text up to that point,
allowing you to easily see where in the template the error occurred.
- **Include Files**. Templates can include other templates. You can specify
a list of search directories for the include files, allowing you to put include files in a variety of
locations, and have include files in one directory that override another directory.
- **Custom Tags**. You can define named fragments that you can include at will, 
and you can define these fragments in include files. To use these fragments, you just
use the name of the fragment as the tag. This essentially gives you the ability to create your own template
language. When you use a custom tag, you can also include parameters that will 
replace placeholders in your fragment, giving you even more power to your custom tags. 

Using other go libraries, you can have your templates compile when they are changed, 
use buffer pools to increase performance, write to io.Writers, and more. Since the
templates become go code, you can do what you imagine.


## Installation

```shell
go get -u github.com/goradd/got/got
```

GoT will format any resulting go code using `go fmt`, but we recommend installing `goimports` 
and passing it the -i flag on the command line to use goimports instead, since that will add the
additional service of fixing up the import lines of any generated go files.
```shell
go get -u golang.org/x/tools/cmd/goimports
```

## Usage

```shell
got [options] [files]

options:
	- o: The output directory. If not specified, files will be output at the same location as the corresponding template.
	- t fileType: If set, will process all files in the current directory with this suffix. If not set, you must specify the files at the end of the command line.
	- i: Run `goimports` on the output files, rather than `go fmt`
	- I directories and/or files:  A list of directories and/or files. If a directory, it is used as 
		the search path for include files. If a file, it is automatically added to the front of every file that is
		processed.  Directories are searched in the order specified and first matching file will be used. It
		will always look in the current directory last unless the current directory is specified
		in the list in another location. Relative paths must start with a dot (.) or double-dot (..).
        Directories can start with a module name, and based on the current directory, the correct go.mod
        file will be searched to know where to look for include files.
	- d directory: When using the -t option, will specify a directory to search.

When running on go 1.11, if a path described above starts with a module path, the actual disk location 
will be substituted. On Go 1.10 or lower, a path will be compared against all import paths. Since the import paths
depend on the directory where this is run, it is important to make sure the current working directory is the same as
the application source you want to inspect.

examples:
	got -t got -i -o ../templates
	got -I .;../tmpl;example.com/projectTemplates file1.tmpl file2.tmpl
```

## Basic Syntax
Template tags start with `{{` and end with `}}`.

A template starts in go mode. To send simple text or html to output, surround the text with `{{` and `}}` tags
with a space or newline separating the tags from the surrounding text. Inside the brackets you will be in 
text mode.
From within text mode, you can send out a go value by surrounding the go code with `{{` and `}}` tags without spaces
separating the go code from the brackets.

In the resulting Go code, text will get written to output by calling:

 ```_, err = io.WriteString(_w, <text>)``` 

Got assumes that the `_w` variable
is available and satisfies the io.Writer interface
and optionally the io.StringWriter interface.
Usually you would do this by declaring a function at the top of your template that looks like this:

``` func f(_w io.Writer) (err error) ```

After compiling the template output together with your program, you call
this function to get the template output. 

At a minimum, you will need to import the "io" package into the file with your template function.
Depending on what tags you use, you might need to add 
additional items to your import list. Those are mentioned below with each tag.

### Example
Here is how you might create a very basic template. For purposes of this example, we will call the file
`example.got` and put it in the `template` package, but you can name the file and package whatever you want.
```
package template

import "io"

func OutTemplate(_w io.Writer) (err error) {
	var world string = "World"
{{
<p>
    Hello {{world}}!
</p>
}}
  return // make sure the error gets returned
}
```

To compile this template, call got:
```shell
got example.got
```

This will create an `example.go` file, which you include in your program. You then can call the function
you declared:
```go
package main

import (
	"io"
	"os"
	"mypath/template"
)

func main() {
	var b bytes.Buffer 
	_ = template.OutTemplate(b)
	_,_ = b.WriteTo(os.Stdout)
}
```

This simple example shows a mix of go code and template syntax in the same file. Using GoT's include files,
you can separate your go code from template code if you want.

## Template Syntax

The following describes how open tags work. Most tags end with a ` }}`, unless otherwise indicated.
Many tags have a short and a long form. Using the long form does not impact performance, its just there
to help your templates have some human readable context to them if you want that.

### Static Text
    {{<space or newline>   Begin to output text as written.
    {{! or {{esc           Html escape the text. Html reserved characters, like < or > are turned into html entities first.
                           This happens when the template is compiled, so that when the template runs, the string will already be escaped. 
    {{h or {{html          Html escape and html format double-newlines into <p> tags.
    {{t or {{translate     Send the text to a translator


The `{{!` tag requires you to import the standard html package. `{{h` requires both the html and strings packages.

`{{t` will wrap the static text with a call to t.Translate(). Its up to you to define this object and make it available to the template. The
translation will happen during runtime of your program. We hope that a future implementation of GoT could
have an option to send these strings to an i18n file to make it easy to send these to a translation service.

#### Example
In this example file, note that we start in Go mode, copying the text verbatim to the template file.
```
package test

import (
	"io"
	"fmt"
)

type Translater interface {
	Translate(string) string
}

func staticTest(_w io.Writer) {
{{
<p>
{{! Escaped html < }}
</p>
}}

{{h
	This is text that is both escaped.
	 
	And has html paragraphs inserted.
}}

}

func translateTest(t Translater, buf *bytes.Buffer) {

{{t Translate me to some language }}

{{!t Translate & escape me > }}

}
```

### Switching Between Go Mode and Template Mode
From within any static text context described above you can switch into go context by using:

    {{g or {{go     Change to straight go code.
    
Go code is copied verbatim to the final template. Use it to set up loops, call special processing functions, etc.
End go mode using the `}}` closing tag. You can also include any other GoT tag inside of Go mode,
meaning you can nest Go mode and all the other template tags.

#### Example
```
// We start a template in Go mode. The next tag switches to text mode, and then nests
// switching back to go mode.
{{ Here 
{{go 
io.WriteString(_w, "is") 
}} some code wrapping text escaping to go. }}
```

### Dynamic Text
The following tags are designed to surround go code that returns a go value. The value will be 
converted to a string and sent to the buf. The go code could be a static value, or a function
that returns a value.

    Tag                       Description                                Example
    
    {{=, {{s, or  {{string    Send a go string to output                 {{= fmt.Sprintf("I am %s", sVar) }}
    {{i or {{int              Send an int to output                      {{ The value is: {{i iVar }} }}
    {{u or {{uint             Send an unsigned Integer                   {{ The value is: {{u uVar }} }}
    {{f or {{float            Send a floating point number               {{ The value is: {{f fVar }} }}
    {{b or {{bool             A boolean (will output "true" or "false")  {{ The value is: {{b bVar }} }}
    {{w or {{bytes            A byte slice                               {{ The value is: {{w byteSliceVar }} }}
    {{v or {{stringer or      Send any value that implements             {{ The value is: {{objVar}} }}
       {{goIdentifier}}       the Stringer interface.


This last tag can be slower than the other tags since it uses fmt.Sprint() internally, 
so if this is a heavily used template,  avoid it. Usually you will not notice a speed difference though,
and the third option can be very convenient. This third option is simply any go variable surrounded by mustaches 
with no spaces.

The i, u, and f tags use the strconv package, so be sure to include that in your template.

#### Escaping Dynamic Text

Some value types potentially could produce html reserved characters. The following tags will html escape
the output.

    {{!=, {{!s or {{!string    HTML escape a go string
    {{!w or {{!bytes           HTML escape a byte slice
    {{!v or {{!stringer        HTML escape a Stringer
    {{!h                       Escape a go string and html format breaks and newlines

These tags require you to import the "html" package. The `{{!h` tag also requires the "strings" package.

#### Capturing Errors

These tags will receive two results, the first a value to send to output, and the second an error
type. If the error is not nil, processing will stop and the error will be returned by the template function. Therefore, these
tags expect to be included in a function that returns an error. Any template text
processed so far will still be sent to the output buffer.

    {{=e, {{se, {{string,err      Output a go string, capturing an error
    {{!=e, {{!se, {{!string,err   HTML escape a go string and capture an error
    {{ie or {{int,err             Output a go int and capture an error
    {{ue or {{uint,err            Output a go uint and capture an error
    {{fe or {{float,err           Output a go float64 and capture an error
    {{be or {{bool,err            Output a bool ("true" or "false") and capture an error
    {{we, {{bytes,err             Output a byte slice and capture an error
    {{!we or {{!bytes,err         HTML escape a byte slice and capture an error
    {{ve, {{stringer,err          Output a Stringer and capture an error
    {{!ve or {{!stringer,err      HTML escape a Stringer and capture an error
    {{e, or {{err                 Execute go code that returns an error, and stop if the error is not nil

##### Example
```go
func Tester(s string) (out string, err error) {
	if s == "bad" {
		err = errors.New("This is bad.")
	}
	return s
}

func OutTemplate(toPrint string, buf bytes.Buffer) error {
{{=e Tester(toPrint) }}
}
```

### Include Files
#### Include a got source file

    {{: "fileName" }} or {{include "fileName" }}   Inserts the given file name into the template.

 The included file will start in whatever mode the receiving template is in, as if the text was inserted
 at that spot, so if the include tags are  put inside of go code, the included file will start in go mode. 
 The file will then be processed like any other got file. Include files can refer to other include files,
 and so are recursive.
 
 Include files are searched for in the current directory, and in the list of include directories provided
 on the command line by the -I option.

Example: `{{: "myTemplate.inc" }}`

#### Include a text file

    {{:! "fileName" }} or {{includeEscaped "fileName" }}   Inserts the given file name into the template and html escapes it.
    {{:h "fileName" }} or {{includeAsHtml "fileName" }}   Inserts the given file name into the template, html escapes it, and converts newlines into html breaks.

Use `{{:!` to include a file that you surround with a `<pre>` tag to include a text file
and have it appear in an html document looking the same. Use `{{:h` to include a file
without the `<pre>` tags, but if the file uses extra spaces for indent, those spaces will
not indent in the html. These kinds of include files will not be searched for GoT commands.
 
### Defined Fragments

Defined fragments start a block of text that can be included later in a template. The included text will
be sent as is, and then processed in whatever mode the template processor is in, as if that text was simply
inserted into the template at that spot. You can include the `{{` or `{{g` tags inside of the fragment to
force the processor into the text or go modes if needed. The fragment can be defined
any time before it is included, including being defined in other include files. You can add optional parameters
to a fragment that will be substituted for placeholders when the fragment is used. You can have up to 9
placeholders ($1 - $9). Parameters should be separated by commas, and can be surrounded by quotes if needed.

    {{< fragName }} or {{define fragName }}                  Start a block called "fragName".
    {{< fragName <count>}} or {{define fragName <count>}}    Start a block called "fragName" that will have <count> parameters.
    {{> fragName param1,param2,...}} or                      Substitute this tag for the given defined fragment.
      {{put fragName param1,param2,...}} or just
      {{fragName param1,param2,...}}
    {{>? fragName param1,param2,...}} or                     Substitute this tag for the given defined fragment, but if the fragment is not defined, leave blank.
    {{put? fragName param1,param2,...}}

 
If you attempt to use a fragment that was not previously defined, GoT will panic and stop compiling.

param1, param2, ... are optional parameters that will be substituted for $1, $2, ... in the defined fragment.
If a parameter is not included when using a fragment, an empty value will be substituted for the parameter in the fragment.
Use commas to separate parameters, including empty parameters.

The fragment name is NOT surrounded by quotes, and cannot contain any whitespace in the name. Blocks are ended with a
`{{end fragName}}` tag. The end tag must be just like that, with no spaces after the fragName.

The following fragments are predefined:
* `{{templatePath}}` will result in the full path of the template file being processed
* `{{templateName}}` will produce the base name of the template file being processed, including any extensions
* `{{templateRoot}}` will produce the base name of the template file being processed without any extensions
* `{{templateParent}}` will produce the directory name of the template file being processed, without the preceeding path
* `{{outPath}}` will result in the full path of the output file being written
* `{{outName}}` will produce the base name of the output file being written, including any extensions
* `{{outRoot}}` will produce the base name of the output file being written without any extensions
* `{{outParent}}` will produce the directory name of the output file being written, without the preceeding path

Note that if you are looking at these from an included file, these will be the parent file. Multiple
levels of includes will return the information for the top level file being processed. 

#### Example
```

{{< hFrag }}
<p>
This is my html body.
</p>
{{end hFrag}}

{{< writeMe 2}}
{{// The g tag here forces us to process the text as go code, no matter where the fragment is included }}
{{g 
if "$2" != "" {
	io.WriteString(_w, "$1")
}
}}
{{end writeMe}}


func OutTemplate(_w io.Writer) (err error) {
{{
	<html>
		<body>
			{{> hFrag }}
		</body>
	</html>
}}

{{writeMe "Help Me!", a}}
{{writeMe "Help Me!", }}
 return
}
```

### Comment Tags

    {{# or {{//                           Comment the template. This is removed from the compiled template.

These tags and anything enclosed in them is removed from the compiled template.

### Go Block Tags
    
    {{if <go condition>}}<text block>{{if}}                                     This is a convenience tag for surrounding text with a go "if" statement.
    {{if <go condition>}}<text block>{{else}}<text block>{{if}}                      Go "if" and "else" statement.
    {{if <go condition>}}<text block>{{elseif <go condition>}}<text block>{{if}}    Go "if" and "else if" statement.
    {{for <go condition>}}<text block>{{for}}                                   This is a convenience tag for surrounding text with a go "for" statement.

These tags are substitutes for switching into GO mode and using a `for` or `if` statements. 
<text block> will be in text mode to begin with, so that whatever you put there
will be output, but you can switch to go mode if needed.

####Example

```
{{
{{for num,item := range items }}
<p>Item {{num}} is {{item}}</p>
{{for}}
}}
```
### Join Tags

    {{join <slice>, <string>}}<text block>{{join}}    Joins the items of a slice with a string.

Join will execute the <text block> for each item of <slice>. Within <text block> the variable
"_i" will be an integer representing the index of the slice item, and "_j" will be the
item itself. <text block> starts in text mode, but you can put GoT commands in it. <string> will be output
between the output of each item, creating an effect similar to joining a slice of strings.


####Example

```
{{g
  items := []string{"a", "b", "c"}
}}
{{join items,", "}}
{{ {{_i}} = {{_j}}}}
{{join}}
```


### Strict Text Block Tag

From within most of the GoT tags, you can insert another GoT tag. GoT will be looking for these
as it processes text. If you would like to turn off GoT's processing to output text that looks just like a GoT tag,
you can use:

    {{begin *endTag*}} Starts a strict text block and turns off the got parser. 
    
One thing this is useful for is to use Got to generate Got code.
End the block with a `{{end *endTag*}}` tag, where `*endTag*` is whatever you specified in the begin tag. 
There can be no space between the endTag and the final brackets
The following example will output the entire second line of code with no changes, 
including all brackets:

```
{{begin mystrict}}
{{! This is verbatim code }}
{{< all included}}
{{end mystrict}}
```

## Bigger Example

In this example, we will combine multiple files. One, a traditional html template with a place to fill in
some body text. Another, a go function declaration that we will use when we want to draw the template.
The function will use a traditional web server io.Writer pattern, including the use of a context parameter.
Finally, there is an example main.go illustrating how our template function would be called.

### index.html

```html
<!DOCTYPE html>
<html>
    <head>
        <meta charset="utf-8">
    </head>

    <body>
{{# The tag below declares that we want to substitute a named fragment that is declared elsewhere }}
{{put body }}
    </body>
</html>
```

### template.got

```
package main

import {
	"context"
	"bytes"
}


func writeTemplate(ctx context.Context, buf *bytes.Buffer) {

{{# Define the body that will be inserted into the template }}
{{< body }}
<p>
The caller is: {{=s ctx.Value("something") }}
</p>
{{end body}}

{{# include the html template. Since the template is html, we need to put ourselves in static text mode first }}
{{ 
{{include "index.html"}}
}}

}
```

### main.go

```html
type myHandler struct {}

func (h myHandler) ServeHTTP(w http.ResponseWriter, r *http.Request)  {
	ctx :=  context.WithValue(r.Context(), "caller", r.Referer())
	b := new(bytes.Buffer)
	writeTemplate(ctx, b)	// call the got template
	w.Write(b.Bytes())
}


func main() {
	var r myHandler
	var err error

	*local = "0.0.0.0:8000"

	err = http.ListenAndServe(*local, r)

	if err != nil {
		fmt.Println(err)
	}
}
```

To compile the template:

```shell
got template.got
```

Build your application and go to `http://localhost:8000` in your browser, to see your results

## License

Got is licensed under the MIT License.


## Acknowldgements

GoT was influenced by:

- [hero](https://github.com/shiyanhui/hero)
- [fasttemplate](https://github.com/valyala/fasttemplate)
- [Rob Pike's Lexing/Parsing Talk](https://www.youtube.com/watch?v=HxaD_trXwRE)

## Syntax Changes

###v0.10.0
This was a major rewrite with the following changes:
- defined fragments end with {{end fragName}} tags, rather than {{end}} tags
- {{else if ...}} is now {{elseif ...}}
- {{join }} tag will join items with a string
- The backup tag {{- has been removed
- Reorganized the lexer and parser to be easier to debug
- Added many more unit tests. Code coverage > 90%.
- The output is sent to an io.Writer called _w. This allows more flexible use of the templates, and the ability to wrap them with middleware