package clipboard

import "errors"

func errorsAsStd(err error, target any) bool { return errors.As(err, target) }
