package com.ol1n.pass

import android.content.Context
import android.content.Intent
import android.util.Log
import io.flutter.embedding.engine.plugins.FlutterPlugin
import io.flutter.plugin.common.MethodCall
import io.flutter.plugin.common.MethodChannel

/**
 * MethodChannel `ol1n_pass/identity`:
 *  - `readLocal()`        → this app's stored id or null
 *  - `readShared()`       → first id found in a sibling Ol1n app, or null
 *  - `writeLocal({id})`   → store locally (visible to siblings via our provider)
 */
class Ol1nPassPlugin : FlutterPlugin, MethodChannel.MethodCallHandler {
    private lateinit var channel: MethodChannel
    private lateinit var context: Context

    override fun onAttachedToEngine(binding: FlutterPlugin.FlutterPluginBinding) {
        context = binding.applicationContext
        channel = MethodChannel(binding.binaryMessenger, CHANNEL)
        channel.setMethodCallHandler(this)
    }

    override fun onDetachedFromEngine(binding: FlutterPlugin.FlutterPluginBinding) {
        channel.setMethodCallHandler(null)
    }

    override fun onMethodCall(call: MethodCall, result: MethodChannel.Result) {
        when (call.method) {
            "readLocal" -> result.success(PassIdentityStorage.read(context))
            "readShared" -> result.success(readShared())
            "writeLocal" -> {
                val id = call.argument<String>("id")
                if (id.isNullOrEmpty()) {
                    result.error("bad_args", "id is required", null)
                } else {
                    PassIdentityStorage.write(context, id)
                    result.success(null)
                }
            }
            else -> result.notImplemented()
        }
    }

    /** Sibling Ol1n apps = packages exporting a service for ACTION_IDENTITY. */
    private fun siblingPackages(): List<String> {
        val intent = Intent(PassIdentityProvider.ACTION_IDENTITY)
        val resolved = try {
            context.packageManager.queryIntentServices(intent, 0)
        } catch (e: Exception) {
            Log.w(TAG, "queryIntentServices failed: ${e.message}")
            emptyList()
        }
        return resolved
            .mapNotNull { it.serviceInfo?.packageName }
            .filter { it != context.packageName }
            .distinct()
    }

    private fun readShared(): String? {
        for (pkg in siblingPackages()) {
            val uri = PassIdentityProvider.uriFor(pkg)
            try {
                context.contentResolver.query(uri, null, null, null, null)?.use { c ->
                    val col = c.getColumnIndex(PassIdentityStorage.KEY)
                    if (col >= 0 && c.moveToFirst()) {
                        val id = c.getString(col)
                        if (!id.isNullOrEmpty()) return id
                    }
                }
            } catch (e: SecurityException) {
                // Different signing key: not one of ours after all.
                Log.d(TAG, "skip $pkg: ${e.message}")
            } catch (e: Exception) {
                Log.w(TAG, "query $pkg failed: ${e.message}")
            }
        }
        return null
    }

    companion object {
        const val CHANNEL = "ol1n_pass/identity"
        private const val TAG = "ol1n_pass"
    }
}
