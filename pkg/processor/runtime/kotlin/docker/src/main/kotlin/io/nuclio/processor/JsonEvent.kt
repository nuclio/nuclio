/*
Copyright 2018 The Nuclio Authors.

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

package io.nuclio.processor

import java.util.Date

import com.google.gson.annotations.SerializedName
import io.nuclio.TriggerInfo


class JsonEvent : io.nuclio.Event {
    private val body: ByteArray? = null
    @SerializedName("content-type")
    private val contentType: String? = null
    private val headers: Map<String, Any>? = null
    private val fields: Map<String, Any>? = null
    private val id: String? = null
    private val method: String? = null
    private val path: String? = null
    private val url: String? = null
    private val timestamp: Date? = null
    private val trigger: Trigger? = null
    private val shardId: Long = 0
    private val numShards: Long = 0
    private val type: String? = null
    private val typeVersion: String? = null
    private val version: String? = null

    override fun getBody(): ByteArray? {
        return this.body
    }

    override fun getBodyObject(): Any? {
        return this.body
    }

    override fun getContentType(): String? {
        return this.contentType
    }

    override fun getID(): String? {
        return this.id
    }

    override fun getTriggerInfo(): TriggerInfo? {
        return this.trigger
    }

    override fun getHeaders(): Map<String, Any>? {
        return this.headers
    }

    override fun getHeader(key: String): Any? {
        return this.headers!![key]
    }

    override fun getHeaderString(key: String): String? {
        return try {
            this.getHeader(key) as String
        } catch (e: ClassCastException) {
            null
        }

    }

    override fun getHeaderBytes(key: String): ByteArray? {
        val header = this.getHeaderString(key) ?: return null
        return header.toByteArray()
    }

    override fun getHeaderLong(key: String): Long {
        return try {
            this.getHeader(key) as Long
        } catch (e: ClassCastException) {
            0
        }

    }

    override fun getField(key: String): Any? {
        return this.fields!![key]
    }

    override fun getFieldString(key: String): String? {
        return try {
            this.getField(key) as String
        } catch (err: ClassCastException) {
            null
        }

    }

    override fun getFieldBytes(key: String): ByteArray? {
        val value = this.getFieldString(key) ?: return null

        return value.toByteArray()
    }

    override fun getFieldLong(key: String): Long {
        return try {
            this.getField(key) as Long
        } catch (err: ClassCastException) {
            0
        }

    }

    override fun getFields(): Map<String, Any>? {
        return this.fields
    }

    override fun getTimestamp(): Date? {
        return this.timestamp
    }

    override fun getPath(): String? {
        return this.path
    }

    override fun getURL(): String? {
        return this.url
    }

    override fun getMethod(): String? {
        return this.method
    }

    override fun getShardID(): Long {
        return this.shardId
    }

    override fun getTotalNumShards(): Long {
        return this.numShards
    }

    override fun getType(): String? {
        return this.type
    }

    override fun getTypeVersion(): String? {
        return this.typeVersion
    }

    override fun getVersion(): String? {
        return this.version
    }
}

internal class Trigger : TriggerInfo {
    @SerializedName("class")
    private var className: String? = null
    @SerializedName("kind")
    private var kindName: String? = null

    override fun getClassName(): String? {
        return this.className
    }

    override fun getKindName(): String? {
        return this.kindName
    }
}