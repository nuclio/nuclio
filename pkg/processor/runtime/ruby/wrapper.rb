require 'optparse'
require 'socket'
require 'json'

class Logger

  def initialize(socket)
    @socket = socket
  end

  def debug(message, **with)
    log(:debug, message, **with)
  end

  def info(message, **with)
    log(:info, message, **with)
  end

  def warn(message, **with)
    log(:warning, message, **with)
  end

  def error(message, **with)
    log(:error, message, **with)
  end

  def log(level, message, **with)
    log_val = {
        level: level,
        message: message,
        with: with,
        datetime: Time.now.strftime('%Y-%m-%dT%H:%M:%S.%L%z')
    }
    @socket.puts "l#{log_val.to_json}"
  end
end

class Context
  attr_reader :logger

  def initialize(logger)
    @logger = logger
  end
end

if __FILE__ == $0
  options = {}
  OptionParser.new do |opt|
    opt.on('--handler HANDLER') { |o| options[:handler] = o }
    opt.on('--socket-path SOCKET_PATH') { |o| options[:socket_path] = o }
  end.parse!

  file, method_name = options[:handler].split('#')

  require_relative file

  socket = UNIXSocket.new(options[:socket_path])
  logger = Logger.new(socket)
  while event = socket.gets
    begin
      context = Context.new(logger)
      res = send(method_name, context, JSON.parse(event))
      code = 200
    rescue => e
      res = "#{e.backtrace.first}: #{e.message} (#{e.class})\n#{e.backtrace.drop(1).join("\n")}"
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