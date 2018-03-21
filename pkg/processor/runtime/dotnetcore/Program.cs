//  Copyright 2017 The Nuclio Authors.
// 
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
// 
//      http://www.apache.org/licenses/LICENSE-2.0
// 
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

using System;
using System.Threading;
using System.Threading.Tasks;

namespace processor
{
    class Program
    {
        static async Task Main(string[] args)
        {

            var socketPath = args[0];
            var dllPath = @"/opt/nuclio/handler/handler.dll";
            var handler = Environment.GetEnvironmentVariable("NUCLIO_FUNCTION_HANDLER");
            var splittedHandler = handler.Split(':');
            var typeName = splittedHandler[0];
            var methodName = splittedHandler[1];
            var wrapper = new Wrapper(dllPath, typeName, methodName, socketPath);
            await Task.Delay(Timeout.Infinite);
            Console.WriteLine("Exiting...");
        }


    }
}
