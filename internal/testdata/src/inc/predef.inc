{{# Checking that predefines work from included file }}

{{templateName}}
{{templateRoot}}
{{templateParent}}
{{outName}}
{{outRoot}}
{{outParent}}
{{includeName}}
{{includeRoot}}
{{includeParent}}

{{: "predef2.inc" }}

{{# verifying values are restored after the included file }}
{{outName}}
{{outRoot}}
{{outParent}}
{{includeName}}
{{includeRoot}}
{{includeParent}}
