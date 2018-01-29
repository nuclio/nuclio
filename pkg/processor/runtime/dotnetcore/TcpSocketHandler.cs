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
using System.Net;
using System.Net.Sockets;
using System.Text;

namespace processor
{
    

    public class TcpSocketHandler : ISocketHandler
    {
        public event EventHandler MessageReceived;
        private static TcpListener listener;
        private static bool accept = false;
        private int clientPort;


        protected virtual void OnMessageReceived(EventArgs e)
        {
            var handler = MessageReceived;
            if (handler != null)
            {
                handler(this, e);
            }
        }
        
        public TcpSocketHandler(int port, int clientPort)
        {
            this.clientPort = clientPort;
            
            var address = IPAddress.Parse("127.0.0.1");
            listener = new TcpListener(address, port);
            listener.Start();
            accept = true;
        }

        public async void SendMessage(string message)
        {
            var client = new TcpClient("127.0.0.1", clientPort);
            var data = System.Text.Encoding.ASCII.GetBytes(message);
            var stream = client.GetStream();

            // Send the message to the connected TcpServer. 
            await stream.WriteAsync(data, 0, data.Length);
            stream.Dispose();
        }

        public void StopListening()
        {
            accept = false;
            listener.Stop();
        }

        public async void Listen()
        {
            if (listener != null && accept)
            {
                // Continue listening.  
                while (true)
                {
                    var client = await listener.AcceptTcpClientAsync(); // Get the client  
                    var stream = client.GetStream();
                    var buffer = new byte[client.ReceiveBufferSize];

                    await stream.ReadAsync(buffer,0,buffer.Length);
                    
                    var message = Encoding.ASCII.GetString(buffer).TrimEnd('\0');  
                    OnMessageReceived(new MessageEventArgs(){Message = message});

                    stream.Dispose();
                }

            }
        }
    }
}