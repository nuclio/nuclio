using System;
using System.Dynamic.Runtime;
using Newtonsoft.Json;
using nuclio_sdk_dotnetcore;

public class nuclio
{
  public string nucliofunction(Context context, Event eventBase)
  {
    var converter = new ExpandoObjectConverter();
    dynamic obj = JsonConvert.DeserializeObject<ExpandoObject>(json, converter);
    return obj.return_this;
  }
}