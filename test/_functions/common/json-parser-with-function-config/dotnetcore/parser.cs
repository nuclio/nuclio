using System;
using System.Dynamic;
using Newtonsoft.Json.Linq;
using nuclio_sdk_dotnetcore;

public class nuclio
{
  public string parser(Context context, Event eventBase)
  {
      string body = eventBase.GetBody();
      dynamic obj = JObject.Parse(body);
      return obj.return_string;
  }
}