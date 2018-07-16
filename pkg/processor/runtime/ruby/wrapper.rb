require 'optparse'
require 'socket'
require 'json'
require 'base64'

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

class ByteBuffer

  def initialize(bytes)
    @bytes = bytes
  end

  def base64_encode
    Base64.encode64(@bytes)
  end
end

class Response < Struct.new(:body, :headers, :content_type, :status_code, :body_encoding)
  def initialize(body, headers: {}, content_type: 'text/plain', status_code: 200, body_encoding: 'text')
    super(body, headers, content_type, status_code, body_encoding)
  end
end

def response_from_output(handler_output)
  if handler_output.is_a?(Response)
    handler_output
  elsif handler_output.is_a?(Array) && handler_output.size == 2
    status_code = handler_output.first
    body, content_type, body_encoding = response_info_from_output(handler_output.last)
    Response.new(body, status_code: status_code, content_type: content_type, body_encoding: body_encoding)
  else
    body, content_type, body_encoding = response_info_from_output(handler_output)
    Response.new(body, status_code: 200, content_type: content_type, body_encoding: body_encoding)
  end
end

def response_info_from_output(handler_output)
  case handler_output
  when String
    return handler_output, 'text/plain', 'text'
  when ByteBuffer
    return handler_output.base64_encode, 'text/plain', 'base64'
  else
    return handler_output.to_json, 'application/json', 'text'
  end
end

if __FILE__ == $0
  options = {}
  OptionParser.new do |opt|
    opt.on('--handler HANDLER') { |o| options[:handler] = o }
    opt.on('--socket-path SOCKET_PATH') { |o| options[:socket_path] = o }
  end.parse!

  file, method_name = options[:handler].split(':')

  require_relative file

  socket = UNIXSocket.new(options[:socket_path])
  logger = Logger.new(socket)
  while event = socket.gets
    begin
      context = Context.new(logger)
      res = send(method_name, context, JSON.parse(event))
      encoded = response_from_output(res)
    rescue => e
      res = "#{e.backtrace.first}: #{e.message} (#{e.class})\n#{e.backtrace.drop(1).join("\n")}"
      encoded = Response.new(res, status_code: 500)
    end
    logger.debug('Response is', response: encoded.to_h)
    socket.puts "r#{encoded.to_h.to_json}"
  end
  socket.close
end