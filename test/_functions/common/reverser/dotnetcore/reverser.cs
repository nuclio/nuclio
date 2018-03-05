using System;
using nuclio_sdk_dotnetcore;

public class nuclio
{
  public string reverser(Context context, Event eventBase)
  {
   var charArray = eventBase.GetBody().ToCharArray();
   Array.Reverse( charArray );
   return new string(charArray);
  }
}