def main(context, event)
  Base64.decode64(event['body']).reverse!
end