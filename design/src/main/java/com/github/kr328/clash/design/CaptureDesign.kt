package com.github.kr328.clash.design

import android.content.Context
import android.text.InputType
import android.view.View
import android.widget.Button
import android.widget.EditText
import android.widget.LinearLayout
import android.widget.TextView
import com.github.kr328.clash.design.databinding.DesignSettingsCommonBinding
import com.github.kr328.clash.design.util.applyFrom
import com.github.kr328.clash.design.util.bindAppBarElevation
import com.github.kr328.clash.design.util.layoutInflater
import com.github.kr328.clash.design.util.root
import com.google.android.material.switchmaterial.SwitchMaterial

class CaptureDesign(context: Context) : Design<CaptureDesign.Request>(context) {
    sealed class Request {
        object Save : Request()
        object Stop : Request()
        object Apps : Request()
        object ExportCA : Request()
        object InstallCA : Request()
        object Refresh : Request()
        object Clear : Request()
        object CopyDiagnostics : Request()
        data class Detail(val id: Long) : Request()
        data class Menu(val id: Long) : Request()
    }
    private val binding = DesignSettingsCommonBinding.inflate(context.layoutInflater, context.root, false)
    override val root: View get() = binding.root
    private val spacing = (16 * context.resources.displayMetrics.density).toInt()
    private val content = LinearLayout(context).apply {
        orientation = LinearLayout.VERTICAL
        setPadding(spacing, spacing, spacing, spacing)
    }
    val enabled = SwitchMaterial(context).apply { setText(R.string.capture_enable) }
    val https = SwitchMaterial(context).apply { setText(R.string.capture_https) }
    val tls12Only = SwitchMaterial(context).apply { setText(R.string.capture_tls12_only) }
    val domains = input(R.string.capture_domains, "api.example.com\n*.example.org")
    val paths = input(R.string.capture_paths, "re:(?i)scan\n/api/*\n/login")
    private val apps = TextView(context)
    private val status = TextView(context)
    private val diagnostics = TextView(context).apply { setTextIsSelectable(true) }
    private val records = LinearLayout(context).apply { orientation = LinearLayout.VERTICAL }
    private var signature = ""

    init {
        binding.surface = surface
        binding.activityBarLayout.applyFrom(context)
        binding.scrollRoot.bindAppBarElevation(binding.activityBarLayout)
        binding.content.addView(content)
        text(R.string.capture_explanation)
        content.addView(enabled)
        content.addView(https)
        content.addView(tls12Only)
        text(R.string.capture_tls12_help)
        text(R.string.capture_domains)
        content.addView(domains)
        text(R.string.capture_paths)
        content.addView(paths)
        content.addView(apps)
        button(R.string.capture_apps, Request.Apps)
        button(R.string.capture_save, Request.Save)
        button(R.string.capture_stop, Request.Stop)
        text(R.string.capture_tls_help)
        button(R.string.capture_export_ca, Request.ExportCA)
        button(R.string.capture_install_ca, Request.InstallCA)
        content.addView(status)
        text(R.string.capture_diagnostics)
        button(R.string.capture_copy_diagnostics, Request.CopyDiagnostics)
        content.addView(diagnostics)
        button(R.string.capture_refresh, Request.Refresh)
        button(R.string.capture_clear, Request.Clear)
        content.addView(records)
    }
    private fun input(label: Int, placeholder: String) = EditText(context).apply {
        hint = placeholder
        contentDescription = context.getString(label)
        inputType = InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_FLAG_MULTI_LINE or InputType.TYPE_TEXT_FLAG_NO_SUGGESTIONS
        minLines = 2
        maxLines = 5
        setHorizontallyScrolling(false)
    }
    private fun text(res: Int) { content.addView(TextView(context).apply { setText(res); setPadding(0, spacing, 0, spacing / 2) }) }
    private fun button(res: Int, request: Request) { content.addView(Button(context).apply { setText(res); setOnClickListener { requests.trySend(request) } }) }
    fun setApps(count: Int) { apps.text = context.getString(R.string.capture_apps_count, count) }
    fun status(value: String) { status.text = value }
    fun diagnostics(value: String) { diagnostics.text = value }
    fun showRecords(rows: List<Pair<Long, String>>) {
        val next = rows.toString()
        if (next == signature) return
        signature = next
        records.removeAllViews()
        if (rows.isEmpty()) records.addView(TextView(context).apply { setText(R.string.capture_empty) })
        rows.forEach { (id, label) ->
            records.addView(Button(context).apply {
                text = label
                isAllCaps = false
                gravity = android.view.Gravity.START or android.view.Gravity.CENTER_VERTICAL
                setOnClickListener { requests.trySend(Request.Detail(id)) }
                setOnLongClickListener { requests.trySend(Request.Menu(id)); true }
            })
        }
    }
}
