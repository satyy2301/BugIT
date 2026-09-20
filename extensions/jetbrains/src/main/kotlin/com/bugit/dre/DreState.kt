package com.bugit.dre

import com.google.gson.JsonParser

object DreState {
    @Volatile
    var title: String = "No snapshot loaded"
    @Volatile
    var events: List<EventRow> = emptyList()

    data class EventRow(val index: Int, val service: String, val summary: String, val isError: Boolean)

    fun applyIdeJson(json: String) {
        val root = JsonParser.parseString(json).asJsonObject
        val manifest = root.getAsJsonObject("manifest")
        val incident = manifest?.getAsJsonObject("incident")
        title = incident?.get("title")?.asString ?: manifest?.get("id")?.asString ?: "DRE snapshot"
        val arr = root.getAsJsonArray("events") ?: return
        events = arr.map { el ->
            val o = el.asJsonObject
            EventRow(
                index = o.get("index")?.asInt ?: 0,
                service = o.get("service")?.asString ?: "unknown",
                summary = o.get("summary")?.asString ?: "",
                isError = o.get("is_error")?.asBoolean ?: false,
            )
        }
    }
}
