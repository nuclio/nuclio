#
# Demonstrates running a shell script. In this case, ImageMagick is installed on build and "convert"
# is called for each event with stdin as the input (by default, this is fed with the event body).
#
# NOTE:
#
# This can be achieved without a wrapper script by specifying the "convert" binary as the handler. To do this
# with nuctl you would run (pass --platform local if you're using the local platform):
#
# nuctl deploy -p /dev/null convert \
#     --runtime shell \
#     --handler convert \
#     --runtime-attrs '{"arguments": "- -resize 50% fd:1"}' \
#     --build-command "apk --update --no-cache add imagemagick"
#
# Doing so gives you much greater flexibility than a wrapper script because the arguments can be changed per event.
# If X-nuclio-arguments does not exist in the event headers, the default arguments passed to convert tells it to
# reduce the image to 50%. To run any other mode or any other setting, simply provide this header (note that this is
# unsanitized). For example, to reduce the received image to 10% of its size, set X-nuclio-arguments to
# "- -resize 10% fd:1"
#

# @nuclio.configure
#
# function.yaml:
#   apiVersion: "nuclio.io/v1beta1"
#   kind: "Function"
#   spec:
#     runtime: "shell"
#     handler: "img-convert.sh:main"
#
#     build:
#       commands:
#       - "apk --update --no-cache add imagemagick"
#

convert - -resize 50% fd:1