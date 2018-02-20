using System;
using System.Collections.Generic;
using System.Linq;
using nuclio_sdk_dotnetcore;

public class nuclio
{
    public object outputter(Context context, Event eventBase)
    {
        if (eventBase.Method != "POST")
            return eventBase.Method;

        var body = eventBase.Body;
        switch (body)
        {
            case "return_string":
                return "a string";
            case "log":
                context.Logger.Log(Logger.LogLevel.Debug, "Debug message");
                context.Logger.Log(Logger.LogLevel.Info, "Info message");
                context.Logger.Log(Logger.LogLevel.Warning, "Warn message");
                context.Logger.Log(Logger.LogLevel.Error, "Error message");
                return "returned logs";
            case "return_response":
                var headers = new Dictionary<string,object>();
                headers.Add("a",eventBase.Headers["A"]);
                headers.Add("b",eventBase.Headers["B"]);
                headers.Add("h1","v1");
                headers.Add("h2","v2");
                var response = new Response();
                response.Headers = headers;
                response.Body = "response body";
                response.StatusCode = 201;
                response.ContentType = "text/plain; charset=utf-8";
                return response;
            case "panic":
                throw new Exception("Panicking, as per request");
            case "return_fields":
                var fields = eventBase.Fields;
                var listOfFields = new List<string>();
                foreach (var field in fields)
                {
                    listOfFields.Add($"{field.Key}={field.Value}");
                }
                listOfFields.Sort();
                return String.Join(",", listOfFields.Select(x => x.ToString()).ToArray());
            case "return_path":
                return eventBase.Path;
        }
        throw new Exception(string.Empty);

    }
}