# Exporting and Importing in Nuclio

This tutorial guides you through the process of exporting and importing functions and projects.

### In this document
- [Exporting a deployed function](#exporting-a-deployed-function)
- [Importing a function](#importing-a-function)
- [Redeploying an imported function](#redeploying-an-imported-function)
- [Exporting a project](#exporting-a-project)
- [Importing a project](#importing-a-project)

## Exporting a deployed function

Once you have some functions deployed in nuclio (see [Deploying functions](/docs/tasks/deploying-functions.md)), you'll be able to export them for later import in this nuclio system or any other nuclio system. After which, it's recommended that you save the output to a file with a redirection (e.g. `command --with output > file.yaml` ).

In order to export the function, run the command (remember to replace `function-name` with the relevant function name):
```sh
nuctl export function --namespace nuclio function-name
```
The command will print the exported function config to the stdout.

> **Note:** This command by default will export the function in yaml format. However, you can supply the flag `--output json` if you prefer a json output.

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

It is also possible to POST the exported file to the dashboard API. For example with the `http` command-line tool:
```sh
cat path-to-exported-function-file | http post 'http://<nuclio-system-url>/api/functions/?import=true'
```

## Redeploying an imported function

Once you import a function, you'll notice it's state is `imported` and it is not deployed.
To deploy an exported function, use the deploy command and supply the command with exported function config file:
```sh
nuctl deploy --namespace nuclio --file path-to-exported-function-file
```
However, if you already imported the function, you can deploy the imported function with this command:
```sh
nuctl deploy --namespace nuclio imported-function-name
```

## Exporting a project

Exporting a project is done similarly to [exporting a function](#exporting-a-deployed-function):
```sh
nuctl export function --namespace nuclio project-name
```

The output of this command will contain the config for the project, the config for each of the project's functions and the configs for all their function events.  Again, similarly to exporting functions, it's recommended that you save the output to a file with a redirection (e.g. `command --with output > file.yaml` ).
> **Note:** Again similarly to [exporting a function](#exporting-a-deployed-function), this command by default will export the function in yaml format. However, you can supply the flag `--output json` if you prefer a json output.

## Importing a project

Importing a project is done similarly to [importing a function](#importing-a-function):
```sh
nuctl import project --namespace nuclio path-to-exported-project-file

# or

cat path-to-exported-project-file | nuctl import project --namespace nuclio
```

Similarly to importing a function, it is also possible to POST the exported file to the dashboard API. For example with the `http` command-line tool:
```sh
cat path-to-exported-project-file | http post 'http://<nuclio-system-url>/api/projects/?import=true'
```

> **Important Note**: As mentioned, importing a project is a high-level flow with many sub-flows:
> - Importing the project config
> - Importing all the project's functions
> - Importing all the project functions' function events
>
> In order for this flow to run smoothly, if one of the resources fails to import, an error will be printed to the stderr, but the command will still continue to run and try to import as many of the resources as possible.
>
> For example: If the namespace contains a function named `my-func`, but a function named `my-func` already exists in the system (function names are unique namespace-wide, not only project-wide), this function will not be imported(and neither will its function events), but the project as a whole will still be imported as well as any other function and function event present in the config.