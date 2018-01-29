//  Copyright 2017 The Nuclio Authors.
// 
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
// 
//      http://www.apache.org/licenses/LICENSE-2.0
// 
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

using System;
using System.Collections.Generic;
using System.Runtime.Serialization;
using MessagePack;
using MessagePack.Formatters;

namespace processor
{

    [MessagePackObject]
    public abstract class EventBase
    {
        [Key("body")]
        public byte[] body { get; set; }

        [Key("content-type")]
        public string contentType { get; set; }

        [Key("headers")]
        public Dictionary<string, object> headers { get; set; }

        [Key("fields")]
        public Dictionary<string, object> fields { get; set; }

        [Key("size")]
        public long size { get; set; }

        [Key("id")]
        public string id { get; set; }

        [Key("method")]
        public string method { get; set; }

        [Key("path")]
        public string path { get; set; }

        [Key("url")]
        public string url { get; set; }

        [Key("version")]
        public long version { get; set; }

        [Key("timestamp")]
        [MessagePackFormatter(typeof(DateTimeUnixFormatter))]
        public DateTime timestamp { get; set; }

        [Key("trigger")]
        public Trigger trigger { get; set; }

    }

    [MessagePackObject]
    public class Trigger
    {
        [Key("class")]
        public string className { get; set; }

        [Key("kind")]
        public string kind { get; set; }
    }

    public class DateTimeUnixFormatter : IMessagePackFormatter<DateTime>
    {
        public DateTime Deserialize(byte[] bytes, int offset, IFormatterResolver formatterResolver, out int readSize)
        {
            var unixTimestamp = MessagePackBinary.ReadInt32(bytes, offset, out readSize);
            var unixDateTime = new DateTime(1970, 1, 1, 0, 0, 0, 0, System.DateTimeKind.Utc);
            unixDateTime = unixDateTime.AddSeconds(unixTimestamp).ToLocalTime();
            return unixDateTime;
        }

        public int Serialize(ref byte[] bytes, int offset, DateTime value, IFormatterResolver formatterResolver)
        {
            var unixTimestamp = (Int32)(value.Subtract(new DateTime(1970, 1, 1))).TotalSeconds;
            return MessagePackBinary.WriteInt32(ref bytes, offset, unixTimestamp);
        }
    }

    public class Event : EventBase
    {
        internal static Event Deserialize(string eventString)
        {
            var bin = MessagePackSerializer.FromJson(eventString);
            return MessagePackSerializer.Deserialize<Event>(bin);
        }

        internal static string Serialize(Event obj)
        {
            var bin = MessagePackSerializer.Serialize(obj, MessagePack.Resolvers.ContractlessStandardResolver.Instance);
            return MessagePackSerializer.ToJson(bin);
        }
    }
}