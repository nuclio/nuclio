def main(context, event)
  return Response.new(event.method) unless event.method == 'POST'
  case event.body
  when 'return_string'
    'a string'
  when 'return_bytes'
    ByteBuffer.new('bytes')
  when 'return_response'
    Response.new('response body', headers: {h1: :v1, h2: :v2}, content_type: 'text/plain', status_code: 201)
  when 'log'
    context.logger.debug('Debug message')
    context.logger.info('Info message')
    context.logger.warn('Warn message')
    context.logger.error('Error message')

    [201, 'returned logs']
  when 'log_with'
    context.logger.error('Error message', source: :rabbit, weight: 7)

    [201, 'returned logs with']
  when 'return_fields'
    event.fields.to_a.map { |field| field.join('=') }.sort.join(',')
  when 'return_path'
    event.path
  when 'return_error'
    raise 'some error'
  else
    raise "Unknown return mode: #{event.body}"
  end
end