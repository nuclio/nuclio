using System;
using nuclio_sdk_dotnetcore;

public class nuclio
{
  public string nucliofunction(ContextBase context, EventBase eventBase)
  {
    var charArray = eventBase.Body.ToCharArray();
    Array.Reverse( charArray );
    return new string(charArray);
  }
}