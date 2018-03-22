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
using System.Net.Sockets;
using System.Text;
using System.Threading.Tasks;
using Nuclio.Sdk;

namespace processor
{
    public class UnixSocketHandler : ISocketHandler
    {
        public event EventHandler MessageReceived;

        private Socket _socket;
        protected virtual void OnMessageReceived(EventArgs e)
        {
            var handler = MessageReceived;
            if (handler != null)
            {
                handler(this, e);
            }
        }

        public UnixSocketHandler(string socketPath)
        {
            ConnectAndListen(socketPath);
        }

        public async void SendMessage(string message)
        {
            var data = System.Text.Encoding.UTF8.GetBytes(message);
            if (_socket != null)
                await _socket.SendAsync(data, SocketFlags.None);
        }

        private async void ConnectAndListen(string socketPath)
        {
            try
            {
                using (_socket = new Socket(AddressFamily.Unix, SocketType.Stream, ProtocolType.Unspecified))
                {
                    var ep = new UnixDomainSocketEndPoint(socketPath);
                    await _socket.ConnectAsync(ep);
                    var clientReceives = Task.Run(async () =>
                    {
                        while (true)
                        {
                            var buffer = new byte[1024];
                            await _socket.ReceiveAsync(new ArraySegment<byte>(buffer), SocketFlags.None);
                            var message = Encoding.UTF8.GetString(buffer);
                            message = message.Substring(0, message.IndexOf('\n'));
                            OnMessageReceived(new MessageEventArgs() { Message = message });
                        }
                    });

                    await clientReceives;
                }
            }
            catch (Exception ex)
            {
                Console.WriteLine("Socket Error: " + ex.Message);
            }
        }
    }
}
