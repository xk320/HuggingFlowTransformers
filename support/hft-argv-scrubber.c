#define _GNU_SOURCE

#include <dlfcn.h>
#include <stddef.h>
#include <stdlib.h>
#include <string.h>
#include <sys/prctl.h>
#include <unistd.h>

typedef int (*hft_main_fn)(int, char **, char **);
typedef int (*hft_libc_start_main_fn)(hft_main_fn, int, char **, void (*)(void), void (*)(void),
				      void (*)(void), void *);

static char hft_title[128] = "HuggingFlowTransformers-runtime";
static hft_main_fn hft_real_main = NULL;
static char **hft_argv_copy = NULL;

static void hft_load_title(void) {
	const char *title = getenv("HFT_PROCESS_TITLE");
	if (title != NULL && title[0] != '\0') {
		strncpy(hft_title, title, sizeof(hft_title) - 1);
		hft_title[sizeof(hft_title) - 1] = '\0';
	}
	(void)prctl(PR_SET_NAME, hft_title, 0, 0, 0);
}

static char **hft_copy_argv(int argc, char **argv) {
	if (argc <= 0 || argv == NULL) {
		return argv;
	}
	char **copy = calloc((size_t)argc + 1, sizeof(char *));
	if (copy == NULL) {
		return argv;
	}
	for (int i = 0; i < argc; i++) {
		if (argv[i] != NULL) {
			copy[i] = strdup(argv[i]);
		}
	}
	copy[argc] = NULL;
	return copy;
}

static void hft_scrub_argv_memory(int argc, char **argv) {
	if (argc <= 0 || argv == NULL || argv[0] == NULL) {
		return;
	}

	char *start = argv[0];
	char *end = argv[0] + strlen(argv[0]);
	for (int i = 1; i < argc; i++) {
		if (argv[i] == NULL) {
			continue;
		}
		char *candidate = argv[i] + strlen(argv[i]);
		if (candidate > end) {
			end = candidate;
		}
	}
	if (end <= start) {
		return;
	}

	size_t total = (size_t)(end - start);
	memset(start, 0, total);
	if (total > 1) {
		strncpy(start, hft_title, total - 1);
	}
	for (int i = 1; i < argc; i++) {
		argv[i] = start;
	}
}

static int hft_main(int argc, char **argv, char **envp) {
	char **effective_argv = hft_argv_copy != NULL ? hft_argv_copy : argv;
	return hft_real_main(argc, effective_argv, envp);
}

int __libc_start_main(hft_main_fn main, int argc, char **ubp_av, void (*init)(void), void (*fini)(void),
		      void (*rtld_fini)(void), void *stack_end) {
	hft_libc_start_main_fn real_start =
		(hft_libc_start_main_fn)dlsym(RTLD_NEXT, "__libc_start_main");
	if (real_start == NULL) {
		_exit(127);
	}

	hft_load_title();
	hft_real_main = main;
	hft_argv_copy = hft_copy_argv(argc, ubp_av);
	hft_scrub_argv_memory(argc, ubp_av);
	return real_start(hft_main, argc, ubp_av, init, fini, rtld_fini, stack_end);
}
