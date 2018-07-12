def print_hello(context, event)
  context.logger.debug('debug log.', method: event['method'], path: event['path'])
  context.logger.info('info log.', method: event['method'], path: event['path'])
  context.logger.warn('warn log.', method: event['method'], path: event['path'])
  context.logger.error('error log.', method: event['method'], path: event['path'])
  "[#{Time.now.ctime}] hello #{event['method']} #{event['path']}"
end