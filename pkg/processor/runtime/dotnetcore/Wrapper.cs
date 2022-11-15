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
using System.Collections.Generic;
using System.Text;
using Nuclio.Sdk;

namespace processor
{

    public class Wrapper
    {
        private delegate object MethodDelegate(Context context, Event eve);
        private delegate void InitContextDelegate(Context context);
        private static MethodDelegate methodDelegate;
        private static string initContextFunctionName = "InitContext";
        private Type methodType;

        private ISocketHandler socketHandler;
        private Context context;

        public Wrapper(string dllPath, string typeName, string methodName, string socketPath)
        {

            CreateTypeAndFunction(dllPath, typeName, methodName);

            InitUnixSocketHandler(socketPath);

            context = new Context();
            context.Logger.LogEvent += LogEvent;

            context.UserData = new Dictionary<string, object>();

            InitContextAndStartUnixSocketHandler();
        }

        private void InitContextAndStartUnixSocketHandler()
        {
            //Run the InitContext method on the function implementation
            try
            {
                ExecuteInitContext();
                socketHandler.SendMessage("{ 'kind': 'wrapperInitialized', 'attributes': {'ready': 'true'}");
            }
            catch (Exception e)
            {
                context.Logger.Error(e.Message);
                context.Logger.Error(e.StackTrace);
                throw new Exception(e.Message);
            }

            StartUnixSocketHandler();
        }

        private void ExecuteInitContext()
        {
            if (methodType == null)
            {
                context.Logger.Error("Failed to execute InitContext(): type not found.");
                throw new Exception("Failed to execute " + initContextFunctionName + ": type not found.");
            }

            var initMethod = methodType.GetMethod(initContextFunctionName);
            if (initMethod != null)
            {
                //invoke the method
                var methodDelegate = (InitContextDelegate)Delegate.CreateDelegate(typeof(InitContextDelegate),
                    null, initMethod, false);
                if (methodDelegate != null)
                {
                    methodDelegate.Invoke(context);
                }
            }
        }

        private void InitUnixSocketHandler(string socketPath)
        {
            socketHandler = new UnixSocketHandler(socketPath);
        }

        private void StartUnixSocketHandler()
        {
            socketHandler.MessageReceived += MessageReceived;
        }

        private void CreateTypeAndFunction(string dllPath, string typeName, string methodName)
        {
            try
            {
                // AssemblyLoadContext.Default.LoadFromAssemblyPath does not load dependency-dlls, so use custom Loader
                var assembly = AssemblyLoader.LoadFromAssemblyPath(dllPath);
                // Get the type to use.
                methodType = assembly.GetType(typeName); // Namespace and class
                // Get the method to call.
                var methodInfo = methodType.GetMethod(methodName);
                // Create the Method delegate
                methodDelegate = (MethodDelegate)Delegate.CreateDelegate(typeof(MethodDelegate), null, methodInfo, true);
            }
            catch (Exception ex)
            {
                var message = $"Error loading function: {ex.Message}, Path: {dllPath}, Type: {typeName}, Function: {methodName}";
                Console.WriteLine(message);
                throw new Exception(message);
            }
        }

        private object InvokeFunction(Context context, Event eve)
        {
            if (eve == null)
            {
                throw new Exception("Event is null");
            }

            var result = methodDelegate.Invoke(context, eve);
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

                try
                {
                    st.Start();
                    var eve = NuclioSerializationHelpers<Event>.Deserialize(msgArgs.Message);
                    var result = InvokeFunction(context, eve);
                    response = CreateResponse(result);
                }
                catch (Exception ex)
                {
                    response = CreateResponse(ex);
                }
                finally
                {
                    st.Stop();
                    var metric = new Metric() { Duration = st.Elapsed.TotalSeconds };
                    socketHandler.SendMessage(string.Join(String.Empty, "m", NuclioSerializationHelpers<Metric>.Serialize(metric), Environment.NewLine));
                    socketHandler.SendMessage(string.Join(String.Empty, "r", NuclioSerializationHelpers<Response>.Serialize(response), Environment.NewLine));
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
            socketHandler.SendMessage(string.Join(String.Empty, "l", NuclioSerializationHelpers<Logger>.Serialize(logger), Environment.NewLine));
        }

    }
}