using System.Collections.Generic;
using MessagePack;
using nuclio_sdk_dotnetcore;

namespace processor
{ 
    public class Response : ResponseBase
    {
        internal static string Serialize(Response obj)
        {
            var bin = MessagePackSerializer.Serialize(obj, MessagePack.Resolvers.ContractlessStandardResolver.Instance);
            return MessagePackSerializer.ToJson(bin);
        }
    }
}