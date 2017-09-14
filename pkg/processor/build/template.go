package build

var processorImageDockerfileTemplate = `
FROM {{processorImageName}}

{{range $sourcePath, $destPath := objectsToCopy}}
COPY {{$sourcePath}} {{$destPath}}
{{end}}

{{if commandsToRun}}
{{range commandsToRun}}
RUN {{.}}
{{end}}
{{end}}

COPY processor /usr/local/bin

CMD [ "processor", "--config", "{{processorConfigPath}}" ]
`
