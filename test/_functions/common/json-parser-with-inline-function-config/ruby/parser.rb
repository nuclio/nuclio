require 'json'

def main(context, event)
  body = JSON.parse(Base64.decode64(event['body']))
  return body['return_this']
end