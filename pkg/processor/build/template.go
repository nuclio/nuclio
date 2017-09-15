package build

var processorImageDockerfileTemplate = `FROM {{baseImageName}}

{{range $sourcePath, $destPath := objectsToCopy}}
COPY {{$sourcePath}} {{$destPath}}
{{end}}

{{if commandsToRun}}
{{range commandsToRun}}
RUN {{.}}
{{end}}
{{end}}

CMD [ "processor", "--config", "{{configPath}}" ]
`
