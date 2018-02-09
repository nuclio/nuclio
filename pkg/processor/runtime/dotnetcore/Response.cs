using System.Collections.Generic;
using MessagePack;

namespace processor
{
    public class ResponseBase
    {
        [Key("body")]
        public byte[] Body { get; set; }
        
        [Key("content_type")]
        public string ContentType { get; set; }
        
        [Key("status_code")]
        public int StatusCode { get; set; }
        
        [Key("headers")]
        public Dictionary<string, object> Headers { get; set; }
        
        [Key("body_encoding")]
        public string BodyEncoding { get { return "base64"; } }

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