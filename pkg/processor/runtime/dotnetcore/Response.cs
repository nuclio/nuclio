using System.Collections.Generic;
using MessagePack;

namespace processor
{
    public class ResponseBase
    {
        public byte[] body { get; set; }
        public string content_type { get; set; }
        public int status_code { get; set; }
        public Dictionary<string, object> headers { get; set; }
        public string body_encoding { get { return "base64"; } }

    }
    public class Response : ResponseBase
    {
        internal static string Serialize(Response obj)
        {
            var bin = MessagePackSerializer.Serialize(obj, MessagePack.Resolvers.ContractlessStandardResolver.Instance);
            return MessagePackSerializer.ToJson(bin);
        }
    }
}