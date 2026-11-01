package com.ol1n.pass

import android.content.ContentProvider
import android.content.ContentValues
import android.database.Cursor
import android.database.MatrixCursor
import android.net.Uri

/**
 * Exposes this app's `pass_user_id` to sibling Ol1n apps.
 *
 * Authority is `<applicationId>.ol1npass` (manifest placeholder), path
 * `identity`, one row with one column [PassIdentityStorage.KEY]. Protected by
 * the signature-level permission `com.ol1n.pass.READ_IDENTITY`, so only apps
 * signed with the same key can query it. Read-only: insert/update/delete throw.
 */
class PassIdentityProvider : ContentProvider() {

    override fun onCreate(): Boolean = true

    override fun query(
        uri: Uri,
        projection: Array<String>?,
        selection: String?,
        selectionArgs: Array<String>?,
        sortOrder: String?,
    ): Cursor? {
        if (uri.lastPathSegment != PATH) return null
        val ctx = context ?: return null
        val cursor = MatrixCursor(arrayOf(PassIdentityStorage.KEY))
        PassIdentityStorage.read(ctx)?.let { cursor.addRow(arrayOf(it)) }
        return cursor
    }

    override fun getType(uri: Uri): String = "vnd.android.cursor.item/vnd.com.ol1n.pass.identity"

    override fun insert(uri: Uri, values: ContentValues?): Uri? =
        throw UnsupportedOperationException("read-only")

    override fun delete(uri: Uri, selection: String?, selectionArgs: Array<String>?): Int =
        throw UnsupportedOperationException("read-only")

    override fun update(
        uri: Uri,
        values: ContentValues?,
        selection: String?,
        selectionArgs: Array<String>?,
    ): Int = throw UnsupportedOperationException("read-only")

    companion object {
        const val PATH = "identity"
        const val AUTHORITY_SUFFIX = ".ol1npass"
        const val ACTION_IDENTITY = "com.ol1n.pass.IDENTITY"

        fun uriFor(packageName: String): Uri =
            Uri.parse("content://$packageName$AUTHORITY_SUFFIX/$PATH")
    }
}
