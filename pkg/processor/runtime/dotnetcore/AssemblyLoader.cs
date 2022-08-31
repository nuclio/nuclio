/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

using System;
using System.IO;
using System.Linq;
using System.Reflection;
using System.Runtime.Loader;
using Microsoft.Extensions.DependencyModel;

// Taken from https://github.com/dotnet/corefx/issues/11639#issuecomment-363441773
public static class AssemblyLoader
{
    public static Assembly LoadFromAssemblyPath(string assemblyFullPath)
    {
        var fileNameWithOutExtension = Path.GetFileNameWithoutExtension(assemblyFullPath);
        var fileName = Path.GetFileName(assemblyFullPath);
        var directory = Path.GetDirectoryName(assemblyFullPath);

        var inCompileLibraries = DependencyContext.Default.CompileLibraries.Any(l => l.Name.Equals(fileNameWithOutExtension, StringComparison.OrdinalIgnoreCase));
        var inRuntimeLibraries = DependencyContext.Default.RuntimeLibraries.Any(l => l.Name.Equals(fileNameWithOutExtension, StringComparison.OrdinalIgnoreCase));

        var assembly = (inCompileLibraries || inRuntimeLibraries)
            ? Assembly.Load(new AssemblyName(fileNameWithOutExtension))
            : AssemblyLoadContext.Default.LoadFromAssemblyPath(assemblyFullPath);

        if (assembly != null)
            LoadReferencedAssemblies(assembly, fileName, directory);

        return assembly;
    }

    private static void LoadReferencedAssemblies(Assembly assembly, string fileName, string directory)
    {
        var filesInDirectory = Directory.GetFiles(directory).Where(x => x != fileName).Select(x => Path.GetFileNameWithoutExtension(x)).ToList();
        var references = assembly.GetReferencedAssemblies();

        foreach (var reference in references)
        {
            if (filesInDirectory.Contains(reference.Name))
            {
                var loadFileName = reference.Name + ".dll";
                var path = Path.Combine(directory, loadFileName);
                var loadedAssembly = AssemblyLoadContext.Default.LoadFromAssemblyPath(path);
                if (loadedAssembly != null)
                {
                    LoadReferencedAssemblies(loadedAssembly, loadFileName, directory);
                }
            }
        }
    }
}
