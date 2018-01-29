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


    public class UnixSocketHandler : ISocketHandler
    {
        public event EventHandler MessageReceived;
        private static Socket socket;
        private static bool accept = false;
        private string clientSocketPath;

        protected virtual void OnMessageReceived(EventArgs e)
        {

            var handler = MessageReceived;
            if (handler != null)
            {
                handler(this, e);
            }
        }

        public UnixSocketHandler(string socketPath, string clientSocketPath)
        {
            this.clientSocketPath = clientSocketPath;
            socket = new Socket(AddressFamily.Unix, SocketType.Stream, ProtocolType.Unspecified);
            accept = true;
            var ep = new UnixEndPoint(socketPath);
            socket.Bind(ep);
            socket.Listen(100);
        }

        public async void SendMessage(string message)
        {
            socket = new Socket(AddressFamily.Unix, SocketType.Stream, ProtocolType.Unspecified);
            var ep = new UnixEndPoint(clientSocketPath);
            var data = System.Text.Encoding.ASCII.GetBytes(message);

            // Send the message to the connected TcpServer. 
            await socket.SendAsync(data, SocketFlags.None);
        }

        public void StopListening()
        {
            accept = false;
            socket.Dispose();
        }

        public async void Listen()
        {
            if (socket != null && accept)
            {
                while (true)
                {
                    var newSocket = await socket.AcceptAsync();
                    byte[] buffer = new byte[newSocket.ReceiveBufferSize];
                    var result = await newSocket.ReceiveAsync(buffer, System.Net.Sockets.SocketFlags.None);
                    var message = Encoding.ASCII.GetString(buffer).TrimEnd('\0');
                    OnMessageReceived(new MessageEventArgs() { Message = message });
                    newSocket.Close();
                }
            }
        }
    }

    internal class UnixEndPoint : EndPoint
    {
        string filename;

        public UnixEndPoint(string filename)
        {
            if (filename == null)
                throw new ArgumentNullException("filename");

            if (filename == "")
                throw new ArgumentException("Cannot be empty.", "filename");
            this.filename = filename;
        }

        public string Filename
        {
            get
            {
                return (filename);
            }
            set
            {
                filename = value;
            }
        }

        public override AddressFamily AddressFamily
        {
            get { return AddressFamily.Unix; }
        }

        public override EndPoint Create(SocketAddress socketAddress)
        {
            /*
             * Should also check this
             *
            int addr = (int) AddressFamily.Unix;
            if (socketAddress [0] != (addr & 0xFF))
                throw new ArgumentException ("socketAddress is not a unix socket address.");
            if (socketAddress [1] != ((addr & 0xFF00) >> 8))
                throw new ArgumentException ("socketAddress is not a unix socket address.");
             */

            if (socketAddress.Size == 2)
            {
                // Empty filename.
                // Probably from RemoteEndPoint which on linux does not return the file name.
                UnixEndPoint uep = new UnixEndPoint("a");
                uep.filename = "";
                return uep;
            }
            int size = socketAddress.Size - 2;
            byte[] bytes = new byte[size];
            for (int i = 0; i < bytes.Length; i++)
            {
                bytes[i] = socketAddress[i + 2];
                // There may be junk after the null terminator, so ignore it all.
                if (bytes[i] == 0)
                {
                    size = i;
                    break;
                }
            }

            string name = Encoding.UTF8.GetString(bytes, 0, size);
            return new UnixEndPoint(name);
        }

        public override SocketAddress Serialize()
        {
            byte[] bytes = Encoding.UTF8.GetBytes(filename);
            SocketAddress sa = new SocketAddress(AddressFamily, 2 + bytes.Length + 1);
            // sa [0] -> family low byte, sa [1] -> family high byte
            for (int i = 0; i < bytes.Length; i++)
                sa[2 + i] = bytes[i];

            //NULL suffix for non-abstract path
            sa[2 + bytes.Length] = 0;

            return sa;
        }

        public override string ToString()
        {
            return (filename);
        }

        public override int GetHashCode()
        {
            return filename.GetHashCode();
        }

        public override bool Equals(object o)
        {
            UnixEndPoint other = o as UnixEndPoint;
            if (other == null)
                return false;

            return (other.filename == filename);
        }
    }
}
