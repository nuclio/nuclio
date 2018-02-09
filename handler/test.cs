using System;
namespace Demo
{
  public class Functions
  {
    public static string CurrentDate()
    {
      return DateTime.Now.ToLongDateString();
    }
  }
}