def print_hello(context, event)
  context.logger.debug('debug log.', method: event['method'], path: event['path'])
  context.logger.info('info log.', method: event['method'], path: event['path'])
  context.logger.warn('warn log.', method: event['method'], path: event['path'])
  context.logger.error('error log.', method: event['method'], path: event['path'])
  # "[#{Time.now.ctime}] hello #{event['method']} #{event['path']}"
  # return 201, 'hello'
  # return Response.new('helloworld', status_code: 404, headers: {nuclio_runtime: :ruby})
  # return ByteBuffer.new('helloworld')
  return {
      code: 200,
      message: 'helloworld'
  }
end