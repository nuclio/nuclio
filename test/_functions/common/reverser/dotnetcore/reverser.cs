using System;
using nuclio_sdk_dotnetcore;

public class nuclio
{
  public string nucliofunction(Context context, Event eventBase)
  {
    context.Logger.Log(Logger.LogLevel.Info, "test");
    var charArray = eventBase.Body.ToCharArray();
    Array.Reverse( charArray );
    return new string(charArray);
  }
}