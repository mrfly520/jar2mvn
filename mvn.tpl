{{range .}}
{{ if .ArtifactId }}
<dependency>
    <groupId>{{.GroupId}}</groupId>
    <artifactId>{{.ArtifactId}}</artifactId>
    <version>{{.Version}}</version>
</dependency>
{{ end }}
{{end}}
