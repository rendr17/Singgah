<script lang="ts">
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
</script>

{#if page.status === 404}
	<h1 class="sg-page-title">Halaman tidak ditemukan</h1>
	<p class="sg-meta">Alamat ini tidak cocok dengan halaman Singgah mana pun.</p>
{:else}
	<h1 class="sg-page-title">Terjadi kesalahan</h1>
	<p class="sg-error" role="alert">
		{page.error?.message ?? 'Permintaan gagal tanpa pesan.'}
	</p>
{/if}

<p><a href={resolve('/')}>← Kembali ke pencarian</a></p>
