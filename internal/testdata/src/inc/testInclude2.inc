{{# This file is starting out in go mode }}
{{
	<p>
	{{! Escaped html < }}
	</p>

{{h
This is text
that is both escaped and has
html paragraphs and breaks inserted.
}}
}}

{{
{{# Including a relative path that should be over-ridden }}
{{: "incSub/incSub.inc" }}
}}

