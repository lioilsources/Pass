package com.ol1n.pass

import android.content.Context

/** This app's own copy of `pass_user_id`, in SharedPreferences. */
internal object PassIdentityStorage {
    const val PREFS = "ol1n_pass"
    const val KEY = "pass_user_id"

    fun read(context: Context): String? =
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
            .getString(KEY, null)
            ?.takeIf { it.isNotEmpty() }

    fun write(context: Context, id: String) {
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
            .edit()
            .putString(KEY, id)
            .apply()
    }
}
