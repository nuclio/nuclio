#
# Copyright 2017 The Nuclio Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

require 'optparse'
require 'socket'
require 'json'
require 'base64'
require 'date'

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

class KeywordStruct < Struct
  def initialize(**kwargs)
    super(*members.map { |k| kwargs[k] })
  end
end

Response = Struct.new(:body, :headers, :content_type, :status_code, :body_encoding) do
  def initialize(body, headers: {}, content_type: 'text/plain', status_code: 200, body_encoding: 'text')
    super(body, headers, content_type, status_code, body_encoding)
  end
end

Event = KeywordStruct.new(:body, :content_type, :headers, :fields, :id, :method, :path, :url, :timestamp, :trigger, :version)

Trigger = KeywordStruct.new(:class_name, :kind)

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
    [handler_output, 'text/plain', 'text']
  when ByteBuffer
    [handler_output.base64_encode, 'text/plain', 'base64']
  else
    [handler_output.to_json, 'application/json', 'text']
  end
end

def parse_event(input)
  json = JSON.parse(input)
  trigger = Trigger.new(class_name: json['trigger']['class'], kind: json['trigger']['kind'])
  Event.new(
    body: Base64.decode64(json['body']),
    content_type: json['content_type'],
    headers: json['headers'],
    fields: json['fields'],
    id: json['id'],
    method: json['method'],
    path: json['path'],
    url: json['url'],
    timestamp: DateTime.strptime(json['timestamp'].to_s, '%s'),
    trigger: trigger,
    version: json['version']
  )
end

if $PROGRAM_NAME == __FILE__
  options = {}
  OptionParser.new do |opt|
    opt.on('--handler HANDLER') { |o| options[:handler] = o }
    opt.on('--socket-path SOCKET_PATH') { |o| options[:socket_path] = o }
  end.parse!

  file, method_name = options[:handler].split(':')

  require_relative file

  socket = UNIXSocket.new(options[:socket_path])
  logger = Logger.new(socket)
  while input = socket.gets
    begin
      context = Context.new(logger)
      event = parse_event(input)
      res = send(method_name, context, event)
      encoded = response_from_output(res)
    rescue StandardError => e
      res = "#{e.backtrace.first}: #{e.message} (#{e.class})\n#{e.backtrace.drop(1).join("\n")}"
      encoded = Response.new(res, status_code: 500)
    end
    logger.debug('Response is', response: encoded.to_h)
    socket.puts "r#{encoded.to_h.to_json}"
  end
  socket.close
end
