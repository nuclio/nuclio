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
using nuclio_sdk_dotnetcore;

public class nuclio
{
  public object helloworld(Context context, Event eventBase)
  {
    context.Logger.Info("This is an unstructured {0}", "log");
    context.Logger.InfoWith("This is a", "structured", "log");
    return new Response() 
      {
		    StatusCode = 200,
		    ContentType = "application/text",
		    Body = "Hello, from nuclio"
	    };
  }
}