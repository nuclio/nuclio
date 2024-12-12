/*
Copyright 2023 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
import io.nuclio.Context;
import io.nuclio.Event;
import io.nuclio.EventHandler;
import io.nuclio.Response;

import org.json.JSONObject;

import java.io.File;
import java.io.FileWriter;
import java.io.IOException;
import java.nio.file.Files;
import java.time.Instant;
import java.util.HashMap;
import java.util.Map;

public class Handler implements EventHandler {

    private static final String EVENTS_LOG_FILE_PATH = "/tmp/events.json";

    @Override
    public Response handleEvent(Context context, Event event) {
        try {
            String triggerKind = ensureString(event.getTriggerInfo().getKindName());
            if (!"http".equals(triggerKind) || invokedByCron(event)) {
                String body = ensureString(event.getBody());
                context.getLogger().debug("Received event with body: " + body);

                // Serialize record
                Map<String, Object> record = new HashMap<>();
                record.put("body", body);
                record.put("headers", ensureHeaders(event.getHeaders()));
                record.put("timestamp", Instant.now().toString());

                String serializedRecord = new JSONObject(record).toString();

                // Append record to log file
                appendToLogFile(serializedRecord);
                return new Response();
            }

            // Read log file and return events
            String encodedEventLog = getEventsLog();
            context.getLogger().debug("Returning events: " + encodedEventLog);
            return new Response().setBody(encodedEventLog).setStatusCode(200);
        } catch (Exception e) {
            context.getLogger().error("Error handling event", e);
            return new Response().setBody(e.toString()).setStatusCode(500);
        }
    }

    private boolean invokedByCron(Event event) {
        String header = getHeader(event, "X-Nuclio-Invoke-Trigger");
        return "cron".equals(header);
    }

    private String ensureString(Object value) {
        if (value instanceof String) {
            return (String) value;
        } else if (value instanceof byte[]) {
            return new String((byte[]) value);
        } else {
            throw new IllegalArgumentException("Unexpected type: " + value.getClass());
        }
    }

    private Map<String, String> ensureHeaders(Map<String, Object> headers) {
        Map<String, String> ensuredHeaders = new HashMap<>();
        headers.forEach((key, value) -> ensuredHeaders.put(ensureString(key), ensureString(value)));
        return ensuredHeaders;
    }

    private String getHeader(Event event, String key) {
        Map<String, Object> headers = event.getHeaders();
        if (headers != null && headers.containsKey(key)) {
            return ensureString(headers.get(key));
        }
        return "";
    }

    private void appendToLogFile(String record) throws IOException {
        File logFile = new File(EVENTS_LOG_FILE_PATH);

        // Read existing content, if any
        String existingData = logFile.exists() ? Files.readString(logFile.toPath()).trim() : "[]";

        // Parse existing data and append the new record
        String updatedData;
        if (existingData.equals("[]")) {
            updatedData = "[" + record + "]";
        } else {
            updatedData = existingData.substring(0, existingData.length() - 1) + "," + record + "]";
        }

        // Write the updated data back to the file
        try (FileWriter fileWriter = new FileWriter(logFile, false)) {
            fileWriter.write(updatedData);
        }
    }

    private String getEventsLog() throws IOException {
        File logFile = new File(EVENTS_LOG_FILE_PATH);
        if (!logFile.exists()) {
            return "[]";
        }

        String data = Files.readString(logFile.toPath()).trim();
        return data.isEmpty() ? "[]" : data;
    }
}
