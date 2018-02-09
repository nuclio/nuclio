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
using System.Reflection;
using System.Runtime.Loader;
using System.Collections.Generic;
using nuclio_sdk_dotnetcore;
namespace processor
{

    public class Wrapper
    {

        private static object typeInstance;
        private static MethodInfo functionInfo;

        private ISocketHandler socketHandler;

        public Wrapper(string dllPath, string typeName, string methodName, int port, int clientPort)
        {
            CreateTypeAndFunction(dllPath, typeName, methodName);
            StartTcpSocketHandler(port, clientPort);
        }

        public Wrapper(string dllPath, string typeName, string methodName, string socketPath, string clientSocketPath)
        {
            CreateTypeAndFunction(dllPath, typeName, methodName);
            StartUnixSocketHandler(socketPath, clientSocketPath);

        }
        private void StartTcpSocketHandler(int port, int clientPort)
        {
            socketHandler = new TcpSocketHandler(port, clientPort);
            socketHandler.MessageReceived += MessageReceived;
            socketHandler.Listen();
        }
        private void StartUnixSocketHandler(string socketPath, string clientSocketPath)
        {
            socketHandler = new UnixSocketHandler(socketPath, clientSocketPath);
            socketHandler.MessageReceived += MessageReceived;
            socketHandler.Listen();
        }

        private void CreateTypeAndFunction(string dllPath, string typeName, string methodName)
        {
            var a = AssemblyLoadContext.Default.LoadFromAssemblyPath(dllPath);
            // Get the type to use.
            var functionType = a.GetType(typeName); // Namespace and class
            // Get the method to call.
            functionInfo = functionType.GetMethod(methodName);
            // Create an instance.
            typeInstance = Activator.CreateInstance(functionType);
        }

        private object InvokeFunction(ContextBase context, EventBase eventBase)
        {
            if (typeInstance == null)
                return null;

            var result = functionInfo.Invoke(typeInstance, new object[] { context, eventBase });
            if (result == null)
                result = string.Empty;
            return result;
        }

        private void MessageReceived(object sender, EventArgs e)
        {
            var msgArgs = e as MessageEventArgs;
            if (msgArgs != null)
            {
                var eve = Event.Deserialize(msgArgs.Message);
                var context = new Context();
                var responseObject = (String)InvokeFunction(context, eve);
                var response = new Response() { Body = responseObject };
                if (response != null)
                {
                    var result = Response.Serialize(response);
                    socketHandler.SendMessage(result);
                }

            }
        }

    }
}