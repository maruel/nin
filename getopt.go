
//go:build nobuild

package ginga


/* include files needed by this include file */

/* macros defined by this include file */

/* types defined by this include file */

/* GETOPT_LONG_OPTION_T: The type of long option */
typedef struct GETOPT_LONG_OPTION_T
{
  stringname;             /* the name of the long option */
  int has_arg;                  /* one of the above macros */
  int *flag;                    /* determines if getopt_long() returns a
                                 * value for a long option; if it is
                                 * non-nil, 0 is returned as a function
                                 * value and the value of val is stored in
                                 * the area pointed to by flag.  Otherwise,
                                 * val is returned. */
  int val;                      /* determines the value to return if flag is
                                 * nil. */
} GETOPT_LONG_OPTION_T

typedef GETOPT_LONG_OPTION_T option

//extern "C"
{

  /* externally-defined variables */
  //extern char *optarg
  //extern int optind
  //extern int opterr
  //extern int optopt

  /* function prototypes */

}

/* END OF FILE getopt.h */

