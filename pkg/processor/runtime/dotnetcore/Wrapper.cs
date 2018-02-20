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
using System.Text;
using nuclio_sdk_dotnetcore;

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

        private void MessageReceived(object sender, EventArgs e)
        {
            var msgArgs = e as MessageEventArgs;
            if (msgArgs != null)
            {
                var st = new System.Diagnostics.Stopwatch();
                Response response = null;
                var context = new Context();
                try
                {
                    st.Start();
                    Console.WriteLine(msgArgs.Message);
                    var eve = Helpers<Event>.Deserialize(msgArgs.Message);
                    context.Logger.LogEvent += LogEvent;
                    var result = InvokeFunction(context, eve);
                    response = CreateResponse(result);
                }
                catch (Exception ex)
                {
                    response = CreateResponse(ex.InnerException);
                }
                finally
                {
                    st.Stop();
                    context.Logger.LogEvent -= LogEvent;
                    var metric = new Metric() { Duration = st.ElapsedTicks };
                    var metricString = "m" + Helpers<Metric>.Serialize(metric) + "\n";
                    socketHandler.SendMessage(metricString);
                    var serializedResponse = "r" + Helpers<Response>.Serialize(response) + "\n";
                    socketHandler.SendMessage(serializedResponse);
                }
            }
        }

        private Response CreateResponse(object value)
        {
            // Create use case for every response type. Currently supported is Response, Exception and primitive types.
            if (value == null)
                value = string.Empty;

            if (value as Response != null)
            {
                var resp = (Response)value;
                ValidateResponse(ref resp);
                return resp;
            }

            var response = new Response();
            response.ContentType = "text/plain";
            response.BodyEncoding = "text";

            if (value as Exception != null)
            {
                response.StatusCode = 500;
                response.Body = ((Exception)(value)).Message;
            }
            else
            {
                response.StatusCode = 200;
                response.Body = value.ToString();
            }

            return response;
        }

        private void ValidateResponse(ref Response response)
        {
            if (string.IsNullOrEmpty(response.ContentType))
            {
                response.ContentType = "text/plain";
            }
            if (string.IsNullOrEmpty(response.BodyEncoding))
            {
                response.BodyEncoding = "text";
            }
            if (response.StatusCode == 0)
            {
                response.StatusCode = 200;
            }
        }

        private void LogEvent(object sender, EventArgs e)
        {
            var logger = (Logger)sender;
            var loggerString = "l" + Helpers<Logger>.Serialize(logger).ToLower() + "\n"; // TODO: Change only the enum serializer to lower
            socketHandler.SendMessage(loggerString);
        }
    }
}