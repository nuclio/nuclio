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
using System.Text;

namespace processor
{

    public class Wrapper
    {

        private static object typeInstance;
        private static MethodInfo functionInfo;

        private ISocketHandler socketHandler;

        public Wrapper(string dllPath, string typeName, string methodName, string socketPath)
        {
            CreateTypeAndFunction(dllPath, typeName, methodName);
            StartUnixSocketHandler(socketPath);

        }

        private void StartUnixSocketHandler(string socketPath)
        {
            socketHandler = new UnixSocketHandler(socketPath);
            socketHandler.MessageReceived += MessageReceived;
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

        private object InvokeFunction(Context context, Event eve)
        {
            try
            {
                if (eve == null)
                {
                    throw new Exception("Event is null");
                }
                if (typeInstance == null)
                    return null;

                var result = functionInfo.Invoke(typeInstance, new object[] { context, eve });
                if (result == null)
                    result = string.Empty;
                return result;
            }
            catch (Exception ex)
            {
                Console.WriteLine("Invocation Error: " + ex.Message);
                return string.Empty;
            }
        }

        private void MessageReceived(object sender, EventArgs e)
        {
            var msgArgs = e as MessageEventArgs;
            if (msgArgs != null)
            {
                var st = new System.Diagnostics.Stopwatch();
                Exception exception = null;
                var responseObject = String.Empty;

                try
                {

                    st.Start();
                    var eve = Helpers<Event>.Deserialize(msgArgs.Message);
                    var context = new Context();
                    responseObject = (String)InvokeFunction(context, eve);
                }
                catch (Exception ex)
                {
                    exception = ex;

                }
                finally
                {
                    st.Stop();
                    var metric = new Metric() { Duration = st.ElapsedTicks };
                    var metricresposne = "m" + Helpers<Metric>.Serialize(metric) + "\n";
                    socketHandler.SendMessage(metricresposne);

                    var response = new Response();
                    if (exception == null)
                    {
                        response.StatusCode = 200;
                    }
                    else
                    {
                        response.StatusCode = 500;
                        responseObject = exception.Message;
                    }

                    response.Body = responseObject;
                    response.ContentType = "text/plain";
                    response.BodyEncoding = "text";
                    var responseStr = "r" + Helpers<Response>.Serialize(response) + "\n";
                    socketHandler.SendMessage(responseStr);
                    Console.WriteLine("Sent: " + responseStr);
                }








            }
        }

    }
}