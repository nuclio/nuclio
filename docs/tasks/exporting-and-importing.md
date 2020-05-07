# Exporting and Importing in Nuclio

This tutorial guides you through the process of exporting and importing functions and projects.

### In this document
- [Exporting a deployed function](#exporting-a-deployed-function)
- [Importing a function](#importing-a-function)
- [Redeploying an imported function](#redeploying-an-imported-function)
- [Exporting a project](#exporting-a-project)
- [Importing a project](#importing-a-project)

## Exporting a deployed function

Once you have some functions deployed in nuclio (see [Deploying functions](/docs/tasks/deploying-functions.md)), you'll be able to export them for later import in this nuclio system or any other nuclio system.

In order to export the function, run the command (remember to replace `function-name` with the relevant function name):
```sh
nuctl export function --namespace nuclio function-name
```
The command will print the exported function config to the stdout.

> **Note:** This command by default will export the function in json format. However, you can supply the flag `--output yaml` if you prefer a yaml output.

Next, it's recommended that you save the output to a file.

## Importing a function

Once you have [exported a function](#exporting-a-deployed-function), you can use the exported function config you saved in a file to import said function.

Either run this command with the file path:
```sh
nuctl import function --namespace nuclio path-to-exported-function-file
```

Or pipe the function config to the command:
```sh
cat path-to-exported-function-file | nuctl import function --namespace nuclio
```

> **Note:** Remember, if you outputted the export in yaml format, to supply the import command with the flag `--input-format yaml`

## Redeploying an imported function

Once you import a function, you'll notice it's state is `imported` and it is not deployed.
To deploy a function during the import, supply the import command with the flag `--deploy`.
However, if you already imported the function without the `--deploy` flag, you can deploy the function with this command:
```sh
nuctl deploy --namespace nuclio imported-function-name
```

## Exporting a project

Exporting a project is done similarly to [exporting a function](#exporting-a-deployed-function):
```sh
nuctl export function --namespace nuclio project-name
```

The output of this command will contain the config for the project, the config for each of the project's functions and the configs for all their function events.
> **Note:** Again similarly to [exporting a function](#exporting-a-deployed-function), this command by default will export the function in json format. However, you can supply the flag `--output yaml` if you prefer a yaml output.

## Importing a project

Importing a project is done similarly to [importing a function](#importing-a-function):
```sh
nuctl import project --namespace nuclio path-to-exported-project-file

# or

cat path-to-exported-project-file | nuctl import project --namespace nuclio
```

> **Note:** Again similarly to [importing a function](#importing-a-function) if you outputted the export in yaml format, remember to supply the import command with the flag `--input-format yaml`

> **Important Note**: As said earlier, importing a project is a procedure with many sub-procedures:
> - Importing the project config
> - Importing all the project's functions
> - Importing all the function's function events
>
> In order for this procedure to run smoothly, if one of the resources fails to import, an error will be printed to the stderr, but the command will still continue to run and try to import as many of the resources as possible.
>
> For example: If the project contains a function named `my-func`, but a function named `my-func` already exists in the system, this function will not be imported(and neither will it's function events), but the project as a whole will still be imported as well as any other function and function event present in the config.