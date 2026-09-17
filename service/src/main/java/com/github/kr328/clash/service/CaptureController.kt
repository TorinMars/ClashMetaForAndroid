package com.github.kr328.clash.service

import android.content.Context
import com.github.kr328.clash.core.bridge.Bridge
import org.json.JSONArray
import org.json.JSONObject

/** Resolve package names in the service process, never trust UIDs supplied by the UI. */
object CaptureController {
    @Synchronized
    fun command(context: Context, command: String, payload: String): String {
        var resolved = payload
        if (command == "configure") {
            try {
                require(payload.length <= 65536) { "Capture configuration too large" }
                val config = JSONObject(payload)
                config.put("uids", JSONArray(resolve(context, config)))
                resolved = config.toString()
            } catch (e: Exception) {
                return JSONObject().put("error", e.message ?: "Invalid applications").toString()
            }
        }
        return Bridge.nativeCapture(command, resolved)
    }

    @Synchronized
    fun revalidate(context: Context) {
        val config = JSONObject(Bridge.nativeCapture("state", "")).optJSONObject("config") ?: return
        if (!config.optBoolean("enabled") || (config.optJSONArray("packages")?.length() ?: 0) == 0) return
        val saved = config.optJSONArray("uids") ?: JSONArray()
        val old = (0 until saved.length()).map { saved.getInt(it) }.toSet()
        val current = try { resolve(context, config).toSet() } catch (_: Exception) { emptySet<Int>() }
        if (current != old) {
            Bridge.nativeCapture("stop", "应用白名单发生变化，请重新选择并应用抓包设置")
        }
    }

    private fun resolve(context: Context, config: JSONObject): List<Int> {
        val packages = config.optJSONArray("packages") ?: JSONArray()
        require(packages.length() <= 100) { "At most 100 applications" }
        return (0 until packages.length()).map {
            context.packageManager.getApplicationInfo(packages.getString(it), 0).uid
        }.distinct()
    }
}
