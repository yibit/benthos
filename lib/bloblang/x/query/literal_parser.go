package query

import (
	"github.com/Jeffail/benthos/v3/lib/bloblang/x/parser"
)

func dynamicArrayParser() parser.Type {
	open, comma, close := parser.Char('['), parser.Char(','), parser.Char(']')
	whitespace := parser.DiscardAll(
		parser.AnyOf(
			parser.NewlineAllowComment(),
			parser.SpacesAndTabs(),
		),
	)
	return func(input []rune) parser.Result {
		res := parser.DelimitedPattern(
			parser.InterceptExpectedError(parser.Sequence(
				open,
				whitespace,
			), "array"),
			parser.AnyOf(
				dynamicLiteralValueParser(),
				parser.InterceptExpectedError(Parse, "object"),
			),
			parser.Sequence(
				parser.Discard(parser.SpacesAndTabs()),
				comma,
				whitespace,
			),
			parser.Sequence(
				whitespace,
				close,
			),
			false, false,
		)(input)
		if res.Err != nil {
			return res
		}

		isDynamic := false
		values := res.Result.([]interface{})
		for _, v := range values {
			if _, isFunction := v.(Function); isFunction {
				isDynamic = true
			}
		}
		if !isDynamic {
			return res
		}

		res.Result = closureFn(func(ctx FunctionContext) (interface{}, error) {
			dynArray := make([]interface{}, len(values))
			var err error
			for i, v := range values {
				if fn, isFunction := v.(Function); isFunction {
					fnRes, fnErr := fn.Exec(ctx)
					if fnErr != nil {
						if recovered, ok := fnErr.(*ErrRecoverable); ok {
							dynArray[i] = recovered.Recovered
							err = fnErr
						}
						return nil, fnErr
					}
					dynArray[i] = fnRes
				} else {
					dynArray[i] = v
				}
			}
			if err != nil {
				return nil, &ErrRecoverable{
					Recovered: dynArray,
					Err:       err,
				}
			}
			return dynArray, nil
		})
		return res
	}
}

func dynamicObjectParser() parser.Type {
	open, comma, close := parser.Char('{'), parser.Char(','), parser.Char('}')
	whitespace := parser.DiscardAll(
		parser.AnyOf(
			parser.NewlineAllowComment(),
			parser.SpacesAndTabs(),
		),
	)

	return func(input []rune) parser.Result {
		res := parser.DelimitedPattern(
			parser.InterceptExpectedError(parser.Sequence(
				open,
				whitespace,
			), "object"),
			parser.Sequence(
				parser.QuotedString(),
				parser.Discard(parser.SpacesAndTabs()),
				parser.Char(':'),
				parser.Discard(whitespace),
				parser.AnyOf(
					dynamicLiteralValueParser(),
					parser.InterceptExpectedError(Parse, "object"),
				),
			),
			parser.Sequence(
				parser.Discard(parser.SpacesAndTabs()),
				comma,
				whitespace,
			),
			parser.Sequence(
				whitespace,
				close,
			),
			false, false,
		)(input)
		if res.Err != nil {
			return res
		}

		isDynamic := false
		values := map[string]interface{}{}
		for _, sequenceValue := range res.Result.([]interface{}) {
			slice := sequenceValue.([]interface{})
			values[slice[0].(string)] = slice[4]
			if _, isFunction := slice[4].(Function); isFunction {
				isDynamic = true
			}
		}
		if !isDynamic {
			res.Result = values
			return res
		}

		res.Result = closureFn(func(ctx FunctionContext) (interface{}, error) {
			dynMap := make(map[string]interface{}, len(values))
			var err error
			for k, v := range values {
				if fn, isFunction := v.(Function); isFunction {
					fnRes, fnErr := fn.Exec(ctx)
					if fnErr != nil {
						if recovered, ok := fnErr.(*ErrRecoverable); ok {
							dynMap[k] = recovered.Recovered
							err = fnErr
						}
						return nil, fnErr
					}
					dynMap[k] = fnRes
				} else {
					dynMap[k] = v
				}
			}
			if err != nil {
				return nil, &ErrRecoverable{
					Recovered: dynMap,
					Err:       err,
				}
			}
			return dynMap, nil
		})
		return res
	}
}

func dynamicLiteralValueParser() parser.Type {
	return parser.AnyOf(
		parser.Boolean(),
		parser.Number(),
		parser.QuotedString(),
		parser.Null(),
		dynamicArrayParser(),
		dynamicObjectParser(),
	)
}

func literalValueParser() parser.Type {
	p := dynamicLiteralValueParser()

	return func(input []rune) parser.Result {
		res := p(input)
		if res.Err != nil {
			return res
		}

		if _, isFunction := res.Result.(Function); isFunction {
			return res
		}

		res.Result = literalFunction(res.Result)
		return res
	}
}
