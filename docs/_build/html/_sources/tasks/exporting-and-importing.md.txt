# Exporting and Importing Resources

This tutorial guides you through the process of using the Nuclio CLI (`nuctl`) to export and import Nuclio functions and projects.

### In this document

- [Exporting deployed functions](#functions-export)
- [Importing functions](#functions-import)
- [Exporting projects](#projects-export)
- [Importing projects](#projects-import)
- [Deploying imported functions](#imported-functions-deploy)

<a id="functions-export"></a>
## Exporting deployed functions

You can  use the Nuclio CLI's `export functions` command (or the `export function` alias) to export the configurations of [deployed Nuclio functions](/docs/tasks/deploying-functions.md) in your environment ("export functions").
You can save the exported configurations, for example, to a file, and [import](#functions-import) them later on any environment that is running Nuclio.

To export a specific function, set the optional `<function>` argument to the name of the function to export:
```sh
nuctl export functions --namespace nuclio <function>
```
For example:
```sh
nuctl export functions --namespace nuclio myfunction
```
By default, if you omit the `<function>` argument, the command exports all deployed Nuclio functions in your environment:
```sh
nuctl export functions --namespace nuclio
```

You can use the `-o|--output` flag to select the output format for the exported configuration &mdash; `"json"` for JSON or `"yaml"`for YAML (default).

The command prints the exported function configurations to the standard output (`stdout`).
It's recommended that you save the output to a configuration file from which you can later import the configuration.
You can do this by redirecting the output of the `export` command to a file.
For example:
```sh
nuctl export functions --namespace nuclio myfunction > myfunction.yaml
```

> **Note:** By default, the `export functions` command doesn't export all the data:
> it "scrubs" sensitive function data (such as authentication information that might be stored in the function triggers) and unnecessary data (such as the namespace).
> You can set the `--no-scrub` flag to override this default behavior and export all function data. 

> **Tip:** Run `nuctl help export functions` for full usage instructions. 

<a id="functions-import"></a>
## Importing functions

You can use the Nuclio CLI's `import functions` command (or the `import function` alias) to import function configurations ("import functions"), typically from previously exported function configurations.
> **Note:** The `import functions` command doesn't deploy the imported functions.
> See [Deploying imported functions](#imported-functions-deploy), which also outlines the option of using the `deploy` command to both import and deploy a function in a single command.

Use either of the following alternatives methods to pass the function configurations to the import command:

- Set the optional `<function-configurations file>` command argument to the path to a YAML or JSON file that contains the configuration of one or more Nuclio functions (as saved, for example, from the output of a previous export command):
    ```sh
    nuctl import functions --namespace nuclio <function-configurations file>
    ```
    For example:
    ```sh
    nuctl import functions --namespace nuclio myfunction.yaml
    ```
- Provide the function configurations in the standard input (`stdin`) and don't pass any arguments to the command.
    For example, the following command passes the configuration via the standard input by piping the contents of a **myfunction.yaml** configuration file to the import command:
    ```sh
    cat myfunction.yaml | nuctl import functions --namespace nuclio
    ```

You can also import function configurations to an instance of the Nuclio dashboard by using an HTTP `POST` command with an `import=true` query string to send a function-configurations file to the dashboard's functions API endpoint &mdash; `/api/functions/`.
You can do this, for example, by using the `http` CLI tool; replace `<function-configurations file>` with the path to a Nuclio function-configurations file, and `<Nuclio dashboard URL>` with the IP address or host name of your Nuclio dashboard:
```sh
cat <function-configurations file> | http post 'http://<Nuclio dashboard URL>/api/functions/?import=true'
```

> **Tip:** Run `nuctl help import functions` for full usage instructions. 

<a id="projects-export"></a>
## Exporting projects

You can use the Nuclio CLI's `export projects` command (or the `export project` alias) to export and save the configurations of Nuclio projects in your environment ("export projects") &mdash; including the configuration of all of the projects' functions, function events, and API gateways.
You can save the exported configurations, for example, to a file, and [import](#projects-import) them later on any environment that is running Nuclio.

To export a specific project, set the optional `<project>` argument to the name of the project to export:
```sh
nuctl export projects --namespace nuclio <project>
```
For example:
```sh
nuctl export projects --namespace nuclio myproject
```
By default, if you omit the `<project>` argument, the command exports all Nuclio projects in your environment:
```sh
nuctl export projects --namespace nuclio
```

You can use the `-o|--output` flag to select the output format for the exported configuration &mdash; `"json"` for JSON or `"yaml"`for YAML (default).

The command prints the exported project configurations to the standard output (`stdout`).
It's recommended that you save the output to a configuration file from which you can later import the configuration.
You can do this by redirecting the output of the `export` command to a file.
For example:
```sh
nuctl export projects --namespace nuclio myproject > myproject.yaml
```
> **Note:** The `export projects` command doesn't export all the data:
> it "scrubs" sensitive function data (such as authentication information that might be stored in the function triggers) and unnecessary data (such as the namespace).
<!-- [IntInfo] Unlike `export functions`, `export projects` doesn't currently
  have a no-scrub flag. -->

> **Tip:** Run `nuctl help export projects` for full usage instructions. 

<a id="projects-import"></a>
## Importing projects

You can use the Nuclio CLI's `import projects` command (or the `import project` alias) to import project configurations ("import projects") &mdash; including the configurations of all of the projects' functions, function events, and API gateways &mdash; typically from previously exported project configurations.
> **Note:** The `import projects` command doesn't deploy the functions in the imported projects.
> See [Deploying imported functions](#imported-functions-deploy).

Use either of the following alternatives methods to pass the project configurations to the import command:

- Set the optional `<project-configurations file>` command argument to the path to a YAML or JSON file that contains the configuration of one or more Nuclio projects (as saved, for example, from the output of a previous export command):
    ```sh
    nuctl import projects --namespace nuclio <project-configurations file>
    ```
    For example:
    ```sh
    nuctl import projects --namespace nuclio myproject.yaml
    ```
- Provide the project configurations in the standard input (`stdin`) and don't pass any arguments to the command.
    For example, the following command passes the configuration via the standard input by piping the contents of a **myproject.yaml** configuration file to the import command:
    ```sh
    cat myproject.yaml | nuctl import projects --namespace nuclio
    ```

You can set the `--skip` flag to the names of projects that are included in the input project configurations but shouldn't be imported (i.e., whose import should be skipped); replace `<projects to skip>` with a comma-separated list of project names:
```sh
nuctl import projects --namespace nuclio --skip <projects to skip> [<project-configurations file>]
```
For example:
```sh
nuctl import projects --namespace nuclio --skip "myproject1,myproject3"
```
<!-- [IntInfo] `import functions` doesn't have a similar `skip` flag. -->

The project display-name configuration (`spec.displayName`) is being deprecated in favor of the project metadata-name configuration (`metadata.name`).
Therefore, by default, when the imported configuration sets `spec.displayName` and doesn't set `metadata.name` or sets it in the form of a UUID, the imported configuration will have a `metadata.name` field with the value of the original `spec.displayName` field and won't have a `spec.displayName` field.
You can bypass this behavior by using the `--skip-transform-display-name` import flag:
```sh
nuctl import projects --namespace nuclio --skip-transform-display-name [<project-configurations file>]
```
For example:
```sh
nuctl import projects --namespace nuclio --skip-transform-display-name
```
> **Warning:** Note that the `spec.displayName` project-configuration field will ultimately be fully deprecated and no longer supported.

You can also import project configurations to an instance of the Nuclio dashboard by using an HTTP `POST` command with an `import=true` query string to send a project-configurations file to the dashboard's projects API endpoint &mdash; `/api/projects/`.
You can do this, for example, by using the `http` CLI tool; replace `<project-configurations file>` with the path to a Nuclio project-configurations file, and `<Nuclio dashboard URL>` with the IP address or host name of your Nuclio dashboard:
```sh
cat <project-configurations file> | http post 'http://<Nuclio dashboard URL>/api/projects/?import=true'
```

> **Note**: As indicated, importing a project configuration also involves importing of all of the project's functions, function events, and API gateways.
> To allow this flow to run smoothly, if one of the resources fails to import, an error is printed to the standard error (`stderr`), but the command continues to run and attempts to import the other relevant resources.
>
> For example, if the imported project contains a function named `myfunction` and a function by this name already exists in another project in the parent namespace, the function (and its function events) won't be imported, because function names in a namespace must be unique.
> But the project as a whole &mdash; including any other functions, function events, and API gateways in the imported configuration &mdash; will still be imported.

> **Tip:** Run `nuctl help import projects` for full usage instructions. 

<a id="imported-functions-deploy"></a>
## Deploying imported functions

The `import functions` and `import projects` commands change the status of the imported functions to the `imported` state, but they don't automatically deploy these functions.
To build and deploy an imported function, you need to use the `deploy` command; replace `<imported function name>` with the name of the imported function to deploy:
```sh
nuctl deploy --namespace nuclio <imported-function name>
```
For example:
```sh
nuctl deploy --namespace nuclio myfunction
```

Alternatively, you can use the `deploy` command with the `-f|--file` flag to import, build, and deploy a function from a function-configuration file without first running an `import` command; replace `<function-configuration file>` with the path to a function-configuration file (typically created from the output of a previous `export` command):
```sh
nuctl deploy --namespace nuclio -f|--file <function-configuration file>
```
For example:
```sh
nuctl deploy --namespace nuclio --file myfunction.yaml
```

> **Tip:** Run `nuctl help deploy` for full usage instructions. 

For more information about deployment of Nuclio functions, see [Deploying Functions](/docs/tasks/deploying-functions.md).

