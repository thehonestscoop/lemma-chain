package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo"
)

type ChainModel struct {
	UID   string `json:"uid"` // We may need to fetch each node's owner name later
	Owner []struct {
		Name string `json:"user.name"`
	} `json:"node.owner"`
	HashID string       `json:"node.hashid"`
	XData  string       `json:"node.xdata"`
	Parent []ChainModel `json:"node.parent"`
	Facet  *string      `json:"node.parent|facet"`
}

func (cm *ChainModel) MarshalJSON() ([]byte, error) {

	data := map[string]interface{}{}

	err := json.Unmarshal([]byte(cm.XData), &data)
	if err != nil {
		return nil, err
	}

	if cm.UID == "" {
		// Due to limited depth, we should ignore this "node": https://github.com/dgraph-io/dgraph/issues/3163
		return []byte("{}"), nil // TODO: BUG HERE
	}

	out := map[string]interface{}{
		"data": data,
	}

	if len(cm.Owner) == 1 {
		out["id"] = "@" + cm.Owner[0].Name + "/" + cm.HashID
	} else {
		out["id"] = cm.HashID
	}

	if len(cm.Parent) != 0 {
		out["ref"] = cm.Parent
	}

	if cm.Facet != nil {
		out["ref_type"] = *cm.Facet
	}

	return json.Marshal(out)
}

func findChainHandler(c echo.Context) error {
	ctx := c.Request().Context()

	if stdQueryTimeout != 0 {
		// Create a max query timeout
		_ctx, cancel := context.WithTimeout(ctx, stdQueryTimeout*time.Millisecond)
		defer cancel()
		ctx = _ctx
	}

	nodeID := strings.TrimSpace(c.Param("*"))

	ownerName, hashID, err := splitNodeID(nodeID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorFmt("can't find ref"))
	}

	var depth int
	_depth := c.QueryParam("depth")
	if _depth != "" {
		depth, err = strconv.Atoi(_depth)
		if err != nil || depth == 0 {
			return c.JSON(http.StatusBadRequest, ErrorFmt("depth query param is malformed"))
		}
	}

	txn := dg.NewReadOnlyTxn()

	// Check if hashID is owned by owner name
	vars := map[string]string{
		"$hashid": hashID,
	}

	q := `
		query withvar($hashid: string) {
			check(func: eq(node.hashid, $hashid)) @normalize {
			    node.owner {
			    	name: user.name 
			    }
			}
		}
	`

	resp, err := txn.QueryWithVars(ctx, q, vars)
	if err != nil {
		if strings.Contains(err.Error(), "context canceled") {
			return c.NoContent(http.StatusNoContent)
		} else if strings.Contains(err.Error(), "context deadline exceeded") {
			return c.NoContent(http.StatusRequestTimeout)
		}
		log.Println(err)
		return c.JSON(http.StatusInternalServerError, ErrorFmt("something went wrong. Try again"))
	}

	type Root struct {
		Check []struct {
			Name *string `json:"name"`
		} `json:"check"`
	}

	var root Root
	err = json.Unmarshal(resp.Json, &root)
	if err != nil {
		log.Println(err)
		return c.JSON(http.StatusInternalServerError, ErrorFmt("something went wrong. Try again"))
	}

	if len(root.Check) == 0 {
		return c.JSON(http.StatusBadRequest, ErrorFmt("can't find ref"))
	}

	actualName := root.Check[0].Name
	if ownerName == nil {
		if actualName != nil {
			return c.JSON(http.StatusBadRequest, ErrorFmt("can't find ref"))
		}
	} else {
		if actualName == nil {
			return c.JSON(http.StatusBadRequest, ErrorFmt("can't find ref"))
		} else if *actualName != *ownerName {
			return c.JSON(http.StatusBadRequest, ErrorFmt("can't find ref"))
		}
	}

	// Find entire chain

	var recursive string
	if _depth == "" {
		recursive = "@recurse(loop:false)"
	} else {
		recursive = fmt.Sprintf("@recurse(depth:%d,loop:false)", depth)
	}

	q = `
		query withvar($hashid: string) {
			chain(func: eq(node.hashid, $hashid)) %s {
				uid # remove after bug: https://github.com/dgraph-io/dgraph/issues/3163
				node.owner
    			user.name
    			node.hashid
    			node.xdata
    			node.parent @facets
  			}
		}
	`

	resp, err = txn.QueryWithVars(ctx, fmt.Sprintf(q, recursive), vars)
	if err != nil {
		if strings.Contains(err.Error(), "context canceled") {
			return c.NoContent(http.StatusNoContent)
		} else if strings.Contains(err.Error(), "context deadline exceeded") {
			return c.NoContent(http.StatusRequestTimeout)
		}
		log.Println(err)
		return c.JSON(http.StatusInternalServerError, ErrorFmt("something went wrong. Try again"))
	}

	type RootChain struct {
		Chain []ChainModel `json:"chain"`
	}

	var rootChain RootChain
	err = json.Unmarshal(resp.Json, &rootChain)
	if err != nil {
		log.Println(err)
		return c.JSON(http.StatusInternalServerError, ErrorFmt("something went wrong. Try again"))
	}

	return c.JSON(http.StatusOK, rootChain.Chain)

}

// splitNodeID returns owner name and hashid
func splitNodeID(nodeID string) (*string, string, error) {

	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return nil, "", errors.New("invalid")
	}

	// Extract owner name
	splits := strings.Split(nodeID, "/")

	if len(splits) == 1 {
		// No owner found
		return nil, splits[0], nil
	}

	if string(splits[0][0]) != "@" {
		// First character of owner name must contain @
		return nil, "", errors.New("invalid")
	}

	ownerID := strings.TrimSpace(strings.TrimPrefix(splits[0], "@"))

	if ownerID == "" {
		return nil, "", errors.New("invalid")
	}

	return &ownerID, splits[1], nil
}

// When setting a depth value:

// spew (main.RootChain) {
//  Chain: ([]main.ChainModel) (len=1 cap=4) {
//   (main.ChainModel) {
//    UID: (string) (len=6) "0x4e78",
//    HashID: (string) (len=10) "7nil4uzhw5",
//    XData: (string) (len=35) "{\"test\":\"xxx\",\"sdagsdg\":[\"sdgsdg\"]}",
//    Parent: ([]main.ChainModel) (len=1 cap=4) {
//     (main.ChainModel) {
//      UID: (string) (len=6) "0x4e77",
//      HashID: (string) (len=9) "roi5euph3",
//      XData: (string) (len=35) "{\"test\":\"xxx\",\"sdagsdg\":[\"sdgsdg\"]}",
//      Parent: ([]main.ChainModel) (len=1 cap=4) {
//       (main.ChainModel) {
//        UID: (string) (len=6) "0x4e76",
//        HashID: (string) (len=9) "6ni0yujh2",
//        XData: (string) (len=35) "{\"test\":\"xxx\",\"sdagsdg\":[\"sdgsdg\"]}",
//        Parent: ([]main.ChainModel) (len=1 cap=4) {
//         (main.ChainModel) {
//          UID: (string) "",
//          HashID: (string) "",
//          XData: (string) "",
//          Parent: ([]main.ChainModel) <nil>,
//          Facet: (*string)(0xc00020d280)((len=8) "required")
//         }
//        },
//        Facet: (*string)(0xc00020d290)((len=8) "required")
//       }
//      },
//      Facet: (*string)(0xc00020d2a0)((len=8) "required")
//     }
//    },
//    Facet: (*string)(<nil>)
//   }
//  }
// }
