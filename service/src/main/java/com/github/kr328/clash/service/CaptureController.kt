package com.github.kr328.clash.service

import android.content.Context
import com.github.kr328.clash.core.bridge.Bridge
import org.json.JSONArray
import org.json.JSONObject
import com.github.kr328.clash.service.model.AccessControlMode

/** Resolve package names in the service process, never trust UIDs supplied by the UI. */
object CaptureController {
    @Volatile var onScopeChanged: (() -> Unit)? = null

    data class VpnScope(val mode: AccessControlMode, val packages: Set<String>, val capturing: Boolean)

    @Synchronized
    fun vpnScope(context: Context, mode: AccessControlMode, packages: Set<String>): VpnScope {
        val config = JSONObject(Bridge.nativeCapture("state", "")).getJSONObject("config")
        val capturing = config.optBoolean("enabled")
        val names = config.optJSONArray("packages") ?: JSONArray()
        val captureApps = (0 until names.length()).map { names.getString(it) }.toSet()
        fun uid(name: String): Int? = try { context.packageManager.getApplicationInfo(name, 0).uid }
            catch (_: android.content.pm.PackageManager.NameNotFoundException) { null }
        val scope = captureVpnScope(mode, packages, context.packageName, capturing, captureApps, ::uid)
        val routing = JSONObject().put("mode", scope.routingMode)
            .put("uids", JSONArray(scope.routingUIDs.toList()))
        val result = JSONObject(Bridge.nativeCapture("routing", routing.toString()))
        check(!result.has("error")) { result.optString("error") }
        return VpnScope(scope.mode, scope.packages, capturing)
    }

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
        val before = if (command == "configure" || command == "stop") JSONObject(Bridge.nativeCapture("state", "")).optJSONObject("config") else null
        val result = Bridge.nativeCapture(command, resolved)
        if (before != null && !JSONObject(result).has("error")) {
            val after = JSONObject(Bridge.nativeCapture("state", "")).getJSONObject("config")
            if (before.optBoolean("enabled") != after.optBoolean("enabled") || before.optJSONArray("packages").toString() != after.optJSONArray("packages").toString()) {
                onScopeChanged?.invoke()
            }
        }
        return result
    }

    @Synchronized
    fun revalidate(context: Context) {
        val config = JSONObject(Bridge.nativeCapture("state", "")).optJSONObject("config") ?: return
        if (!config.optBoolean("enabled") || (config.optJSONArray("packages")?.length() ?: 0) == 0) {
            onScopeChanged?.invoke()
            return
        }
        val saved = config.optJSONArray("uids") ?: JSONArray()
        val old = (0 until saved.length()).map { saved.getInt(it) }.toSet()
        val current = try { resolve(context, config).toSet() } catch (_: Exception) { emptySet<Int>() }
        if (current != old) {
            Bridge.nativeCapture("stop", "应用白名单发生变化，请重新选择并应用抓包设置")
        }
        onScopeChanged?.invoke()
    }

    private fun resolve(context: Context, config: JSONObject): List<Int> {
        val packages = config.optJSONArray("packages") ?: JSONArray()
        require(packages.length() <= 100) { "At most 100 applications" }
        return (0 until packages.length()).map {
            context.packageManager.getApplicationInfo(packages.getString(it), 0).uid
        }.distinct()
    }
}
