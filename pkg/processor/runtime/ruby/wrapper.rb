require 'optparse'
require 'socket'
require 'json'

if __FILE__ == $0
  options = {}
  OptionParser.new do |opt|
    opt.on('--handler HANDLER') { |o| options[:handler] = o }
    opt.on('--socket-path SOCKET_PATH') { |o| options[:socket_path] = o }
  end.parse!

  file, method_name = options[:handler].split('#')

  require_relative file

  socket = UNIXSocket.new(options[:socket_path])
  while event = socket.gets
    begin
      res = send(method_name, event.to_json)
      code = 200
    rescue e
      res = "#{e.backtrace.first}: #{e.message} (#{e.class})\n#{e.backtrace.drop(1).map{ |s| "\t#{s}" }.join("\n")}"
      code = 500
    end
    encoded = JSON.generate(
        {
            body: res,
            status_code: code,
            content_type: 'text/plain',
            headers: {},
            body_encoding: 'text'
        }
    )
    socket.puts "r#{encoded}"
  end
  socket.close
end