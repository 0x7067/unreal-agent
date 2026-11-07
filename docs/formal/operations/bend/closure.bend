# Generated transition equalities are checked against the executable reducer.
import Base
import ./product.bend as P
import ./certificate.bend as C

def edgeCreateErrDone(i: C.Index) -> C.Invariant(P.next(P.fixed(), P.CreateErrDone{}, C.decode(i))):
  match i:
    case C.S0{}: (C.S1{}, {==})
    case C.S1{}: (C.S1{}, {==})
    case C.S2{}: (C.S12{}, {==})
    case C.S3{}: (C.S14{}, {==})
    case C.S4{}: (C.S17{}, {==})
    case C.S5{}: (C.S5{}, {==})
    case C.S6{}: (C.S6{}, {==})
    case C.S7{}: (C.S7{}, {==})
    case C.S8{}: (C.S8{}, {==})
    case C.S9{}: (C.S9{}, {==})
    case C.S10{}: (C.S10{}, {==})
    case C.S11{}: (C.S11{}, {==})
    case C.S12{}: (C.S12{}, {==})
    case C.S13{}: (C.S33{}, {==})
    case C.S14{}: (C.S14{}, {==})
    case C.S15{}: (C.S36{}, {==})
    case C.S16{}: (C.S37{}, {==})
    case C.S17{}: (C.S17{}, {==})
    case C.S18{}: (C.S40{}, {==})
    case C.S19{}: (C.S19{}, {==})
    case C.S20{}: (C.S20{}, {==})
    case C.S21{}: (C.S21{}, {==})
    case C.S22{}: (C.S22{}, {==})
    case C.S23{}: (C.S23{}, {==})
    case C.S24{}: (C.S24{}, {==})
    case C.S25{}: (C.S25{}, {==})
    case C.S26{}: (C.S26{}, {==})
    case C.S27{}: (C.S27{}, {==})
    case C.S28{}: (C.S28{}, {==})
    case C.S29{}: (C.S29{}, {==})
    case C.S30{}: (C.S30{}, {==})
    case C.S31{}: (C.S54{}, {==})
    case C.S32{}: (C.S32{}, {==})
    case C.S33{}: (C.S33{}, {==})
    case C.S34{}: (C.S34{}, {==})
    case C.S35{}: (C.S35{}, {==})
    case C.S36{}: (C.S36{}, {==})
    case C.S37{}: (C.S37{}, {==})
    case C.S38{}: (C.S63{}, {==})
    case C.S39{}: (C.S39{}, {==})
    case C.S40{}: (C.S40{}, {==})
    case C.S41{}: (C.S41{}, {==})
    case C.S42{}: (C.S42{}, {==})
    case C.S43{}: (C.S43{}, {==})
    case C.S44{}: (C.S44{}, {==})
    case C.S45{}: (C.S45{}, {==})
    case C.S46{}: (C.S46{}, {==})
    case C.S47{}: (C.S47{}, {==})
    case C.S48{}: (C.S48{}, {==})
    case C.S49{}: (C.S49{}, {==})
    case C.S50{}: (C.S50{}, {==})
    case C.S51{}: (C.S51{}, {==})
    case C.S52{}: (C.S52{}, {==})
    case C.S53{}: (C.S53{}, {==})
    case C.S54{}: (C.S54{}, {==})
    case C.S55{}: (C.S79{}, {==})
    case C.S56{}: (C.S81{}, {==})
    case C.S57{}: (C.S84{}, {==})
    case C.S58{}: (C.S58{}, {==})
    case C.S59{}: (C.S59{}, {==})
    case C.S60{}: (C.S87{}, {==})
    case C.S61{}: (C.S61{}, {==})
    case C.S62{}: (C.S62{}, {==})
    case C.S63{}: (C.S63{}, {==})
    case C.S64{}: (C.S64{}, {==})
    case C.S65{}: (C.S65{}, {==})
    case C.S66{}: (C.S66{}, {==})
    case C.S67{}: (C.S67{}, {==})
    case C.S68{}: (C.S68{}, {==})
    case C.S69{}: (C.S69{}, {==})
    case C.S70{}: (C.S70{}, {==})
    case C.S71{}: (C.S71{}, {==})
    case C.S72{}: (C.S72{}, {==})
    case C.S73{}: (C.S73{}, {==})
    case C.S74{}: (C.S74{}, {==})
    case C.S75{}: (C.S75{}, {==})
    case C.S76{}: (C.S76{}, {==})
    case C.S77{}: (C.S77{}, {==})
    case C.S78{}: (C.S78{}, {==})
    case C.S79{}: (C.S79{}, {==})
    case C.S80{}: (C.S98{}, {==})
    case C.S81{}: (C.S81{}, {==})
    case C.S82{}: (C.S99{}, {==})
    case C.S83{}: (C.S100{}, {==})
    case C.S84{}: (C.S84{}, {==})
    case C.S85{}: (C.S102{}, {==})
    case C.S86{}: (C.S103{}, {==})
    case C.S87{}: (C.S87{}, {==})
    case C.S88{}: (C.S106{}, {==})
    case C.S89{}: (C.S89{}, {==})
    case C.S90{}: (C.S90{}, {==})
    case C.S91{}: (C.S91{}, {==})
    case C.S92{}: (C.S92{}, {==})
    case C.S93{}: (C.S93{}, {==})
    case C.S94{}: (C.S94{}, {==})
    case C.S95{}: (C.S95{}, {==})
    case C.S96{}: (C.S96{}, {==})
    case C.S97{}: (C.S97{}, {==})
    case C.S98{}: (C.S98{}, {==})
    case C.S99{}: (C.S99{}, {==})
    case C.S100{}: (C.S100{}, {==})
    case C.S101{}: (C.S115{}, {==})
    case C.S102{}: (C.S102{}, {==})
    case C.S103{}: (C.S103{}, {==})
    case C.S104{}: (C.S116{}, {==})
    case C.S105{}: (C.S105{}, {==})
    case C.S106{}: (C.S106{}, {==})
    case C.S107{}: (C.S107{}, {==})
    case C.S108{}: (C.S108{}, {==})
    case C.S109{}: (C.S109{}, {==})
    case C.S110{}: (C.S110{}, {==})
    case C.S111{}: (C.S111{}, {==})
    case C.S112{}: (C.S112{}, {==})
    case C.S113{}: (C.S113{}, {==})
    case C.S114{}: (C.S114{}, {==})
    case C.S115{}: (C.S115{}, {==})
    case C.S116{}: (C.S116{}, {==})
    case C.S117{}: (C.S117{}, {==})
    case C.S118{}: (C.S118{}, {==})
    case C.S119{}: (C.S119{}, {==})
    case C.S120{}: (C.S120{}, {==})
    case C.S121{}: (C.S121{}, {==})
    case C.S122{}: (C.S122{}, {==})
    case C.S123{}: (C.S123{}, {==})
    case C.S124{}: (C.S124{}, {==})
    case C.S125{}: (C.S125{}, {==})
    case C.S126{}: (C.S126{}, {==})

def edgeProcessExit(i: C.Index) -> C.Invariant(P.next(P.fixed(), P.ProcessExit{}, C.decode(i))):
  match i:
    case C.S0{}: (C.S0{}, {==})
    case C.S1{}: (C.S6{}, {==})
    case C.S2{}: (C.S2{}, {==})
    case C.S3{}: (C.S3{}, {==})
    case C.S4{}: (C.S4{}, {==})
    case C.S5{}: (C.S5{}, {==})
    case C.S6{}: (C.S6{}, {==})
    case C.S7{}: (C.S23{}, {==})
    case C.S8{}: (C.S25{}, {==})
    case C.S9{}: (C.S28{}, {==})
    case C.S10{}: (C.S22{}, {==})
    case C.S11{}: (C.S11{}, {==})
    case C.S12{}: (C.S32{}, {==})
    case C.S13{}: (C.S13{}, {==})
    case C.S14{}: (C.S35{}, {==})
    case C.S15{}: (C.S15{}, {==})
    case C.S16{}: (C.S16{}, {==})
    case C.S17{}: (C.S39{}, {==})
    case C.S18{}: (C.S18{}, {==})
    case C.S19{}: (C.S19{}, {==})
    case C.S20{}: (C.S20{}, {==})
    case C.S21{}: (C.S21{}, {==})
    case C.S22{}: (C.S22{}, {==})
    case C.S23{}: (C.S23{}, {==})
    case C.S24{}: (C.S46{}, {==})
    case C.S25{}: (C.S25{}, {==})
    case C.S26{}: (C.S48{}, {==})
    case C.S27{}: (C.S49{}, {==})
    case C.S28{}: (C.S28{}, {==})
    case C.S29{}: (C.S52{}, {==})
    case C.S30{}: (C.S30{}, {==})
    case C.S31{}: (C.S31{}, {==})
    case C.S32{}: (C.S32{}, {==})
    case C.S33{}: (C.S58{}, {==})
    case C.S34{}: (C.S34{}, {==})
    case C.S35{}: (C.S35{}, {==})
    case C.S36{}: (C.S61{}, {==})
    case C.S37{}: (C.S62{}, {==})
    case C.S38{}: (C.S38{}, {==})
    case C.S39{}: (C.S39{}, {==})
    case C.S40{}: (C.S64{}, {==})
    case C.S41{}: (C.S41{}, {==})
    case C.S42{}: (C.S42{}, {==})
    case C.S43{}: (C.S43{}, {==})
    case C.S44{}: (C.S44{}, {==})
    case C.S45{}: (C.S45{}, {==})
    case C.S46{}: (C.S46{}, {==})
    case C.S47{}: (C.S70{}, {==})
    case C.S48{}: (C.S48{}, {==})
    case C.S49{}: (C.S49{}, {==})
    case C.S50{}: (C.S72{}, {==})
    case C.S51{}: (C.S73{}, {==})
    case C.S52{}: (C.S52{}, {==})
    case C.S53{}: (C.S75{}, {==})
    case C.S54{}: (C.S54{}, {==})
    case C.S55{}: (C.S55{}, {==})
    case C.S56{}: (C.S56{}, {==})
    case C.S57{}: (C.S57{}, {==})
    case C.S58{}: (C.S58{}, {==})
    case C.S59{}: (C.S59{}, {==})
    case C.S60{}: (C.S60{}, {==})
    case C.S61{}: (C.S61{}, {==})
    case C.S62{}: (C.S62{}, {==})
    case C.S63{}: (C.S89{}, {==})
    case C.S64{}: (C.S64{}, {==})
    case C.S65{}: (C.S65{}, {==})
    case C.S66{}: (C.S66{}, {==})
    case C.S67{}: (C.S67{}, {==})
    case C.S68{}: (C.S68{}, {==})
    case C.S69{}: (C.S69{}, {==})
    case C.S70{}: (C.S70{}, {==})
    case C.S71{}: (C.S71{}, {==})
    case C.S72{}: (C.S72{}, {==})
    case C.S73{}: (C.S73{}, {==})
    case C.S74{}: (C.S96{}, {==})
    case C.S75{}: (C.S75{}, {==})
    case C.S76{}: (C.S76{}, {==})
    case C.S77{}: (C.S77{}, {==})
    case C.S78{}: (C.S78{}, {==})
    case C.S79{}: (C.S79{}, {==})
    case C.S80{}: (C.S80{}, {==})
    case C.S81{}: (C.S81{}, {==})
    case C.S82{}: (C.S82{}, {==})
    case C.S83{}: (C.S83{}, {==})
    case C.S84{}: (C.S84{}, {==})
    case C.S85{}: (C.S85{}, {==})
    case C.S86{}: (C.S86{}, {==})
    case C.S87{}: (C.S105{}, {==})
    case C.S88{}: (C.S88{}, {==})
    case C.S89{}: (C.S89{}, {==})
    case C.S90{}: (C.S90{}, {==})
    case C.S91{}: (C.S91{}, {==})
    case C.S92{}: (C.S92{}, {==})
    case C.S93{}: (C.S93{}, {==})
    case C.S94{}: (C.S94{}, {==})
    case C.S95{}: (C.S95{}, {==})
    case C.S96{}: (C.S96{}, {==})
    case C.S97{}: (C.S97{}, {==})
    case C.S98{}: (C.S98{}, {==})
    case C.S99{}: (C.S99{}, {==})
    case C.S100{}: (C.S100{}, {==})
    case C.S101{}: (C.S101{}, {==})
    case C.S102{}: (C.S102{}, {==})
    case C.S103{}: (C.S103{}, {==})
    case C.S104{}: (C.S104{}, {==})
    case C.S105{}: (C.S105{}, {==})
    case C.S106{}: (C.S117{}, {==})
    case C.S107{}: (C.S95{}, {==})
    case C.S108{}: (C.S108{}, {==})
    case C.S109{}: (C.S109{}, {==})
    case C.S110{}: (C.S110{}, {==})
    case C.S111{}: (C.S111{}, {==})
    case C.S112{}: (C.S112{}, {==})
    case C.S113{}: (C.S113{}, {==})
    case C.S114{}: (C.S114{}, {==})
    case C.S115{}: (C.S115{}, {==})
    case C.S116{}: (C.S116{}, {==})
    case C.S117{}: (C.S117{}, {==})
    case C.S118{}: (C.S113{}, {==})
    case C.S119{}: (C.S119{}, {==})
    case C.S120{}: (C.S120{}, {==})
    case C.S121{}: (C.S121{}, {==})
    case C.S122{}: (C.S122{}, {==})
    case C.S123{}: (C.S123{}, {==})
    case C.S124{}: (C.S124{}, {==})
    case C.S125{}: (C.S125{}, {==})
    case C.S126{}: (C.S126{}, {==})

def edgeCancelFails(i: C.Index) -> C.Invariant(P.next(P.fixed(), P.CancelFails{}, C.decode(i))):
  match i:
    case C.S0{}: (C.S2{}, {==})
    case C.S1{}: (C.S7{}, {==})
    case C.S2{}: (C.S2{}, {==})
    case C.S3{}: (C.S3{}, {==})
    case C.S4{}: (C.S4{}, {==})
    case C.S5{}: (C.S5{}, {==})
    case C.S6{}: (C.S19{}, {==})
    case C.S7{}: (C.S7{}, {==})
    case C.S8{}: (C.S8{}, {==})
    case C.S9{}: (C.S9{}, {==})
    case C.S10{}: (C.S24{}, {==})
    case C.S11{}: (C.S11{}, {==})
    case C.S12{}: (C.S12{}, {==})
    case C.S13{}: (C.S13{}, {==})
    case C.S14{}: (C.S14{}, {==})
    case C.S15{}: (C.S15{}, {==})
    case C.S16{}: (C.S16{}, {==})
    case C.S17{}: (C.S17{}, {==})
    case C.S18{}: (C.S18{}, {==})
    case C.S19{}: (C.S19{}, {==})
    case C.S20{}: (C.S20{}, {==})
    case C.S21{}: (C.S21{}, {==})
    case C.S22{}: (C.S41{}, {==})
    case C.S23{}: (C.S23{}, {==})
    case C.S24{}: (C.S24{}, {==})
    case C.S25{}: (C.S25{}, {==})
    case C.S26{}: (C.S26{}, {==})
    case C.S27{}: (C.S27{}, {==})
    case C.S28{}: (C.S28{}, {==})
    case C.S29{}: (C.S29{}, {==})
    case C.S30{}: (C.S30{}, {==})
    case C.S31{}: (C.S55{}, {==})
    case C.S32{}: (C.S32{}, {==})
    case C.S33{}: (C.S33{}, {==})
    case C.S34{}: (C.S34{}, {==})
    case C.S35{}: (C.S35{}, {==})
    case C.S36{}: (C.S36{}, {==})
    case C.S37{}: (C.S37{}, {==})
    case C.S38{}: (C.S38{}, {==})
    case C.S39{}: (C.S39{}, {==})
    case C.S40{}: (C.S40{}, {==})
    case C.S41{}: (C.S41{}, {==})
    case C.S42{}: (C.S42{}, {==})
    case C.S43{}: (C.S43{}, {==})
    case C.S44{}: (C.S44{}, {==})
    case C.S45{}: (C.S65{}, {==})
    case C.S46{}: (C.S46{}, {==})
    case C.S47{}: (C.S47{}, {==})
    case C.S48{}: (C.S48{}, {==})
    case C.S49{}: (C.S49{}, {==})
    case C.S50{}: (C.S50{}, {==})
    case C.S51{}: (C.S51{}, {==})
    case C.S52{}: (C.S52{}, {==})
    case C.S53{}: (C.S53{}, {==})
    case C.S54{}: (C.S76{}, {==})
    case C.S55{}: (C.S55{}, {==})
    case C.S56{}: (C.S56{}, {==})
    case C.S57{}: (C.S57{}, {==})
    case C.S58{}: (C.S58{}, {==})
    case C.S59{}: (C.S59{}, {==})
    case C.S60{}: (C.S60{}, {==})
    case C.S61{}: (C.S61{}, {==})
    case C.S62{}: (C.S62{}, {==})
    case C.S63{}: (C.S63{}, {==})
    case C.S64{}: (C.S64{}, {==})
    case C.S65{}: (C.S65{}, {==})
    case C.S66{}: (C.S66{}, {==})
    case C.S67{}: (C.S67{}, {==})
    case C.S68{}: (C.S68{}, {==})
    case C.S69{}: (C.S69{}, {==})
    case C.S70{}: (C.S70{}, {==})
    case C.S71{}: (C.S71{}, {==})
    case C.S72{}: (C.S72{}, {==})
    case C.S73{}: (C.S73{}, {==})
    case C.S74{}: (C.S74{}, {==})
    case C.S75{}: (C.S75{}, {==})
    case C.S76{}: (C.S76{}, {==})
    case C.S77{}: (C.S77{}, {==})
    case C.S78{}: (C.S78{}, {==})
    case C.S79{}: (C.S79{}, {==})
    case C.S80{}: (C.S80{}, {==})
    case C.S81{}: (C.S81{}, {==})
    case C.S82{}: (C.S82{}, {==})
    case C.S83{}: (C.S83{}, {==})
    case C.S84{}: (C.S84{}, {==})
    case C.S85{}: (C.S85{}, {==})
    case C.S86{}: (C.S86{}, {==})
    case C.S87{}: (C.S87{}, {==})
    case C.S88{}: (C.S88{}, {==})
    case C.S89{}: (C.S89{}, {==})
    case C.S90{}: (C.S90{}, {==})
    case C.S91{}: (C.S91{}, {==})
    case C.S92{}: (C.S92{}, {==})
    case C.S93{}: (C.S93{}, {==})
    case C.S94{}: (C.S110{}, {==})
    case C.S95{}: (C.S95{}, {==})
    case C.S96{}: (C.S96{}, {==})
    case C.S97{}: (C.S97{}, {==})
    case C.S98{}: (C.S98{}, {==})
    case C.S99{}: (C.S99{}, {==})
    case C.S100{}: (C.S100{}, {==})
    case C.S101{}: (C.S101{}, {==})
    case C.S102{}: (C.S102{}, {==})
    case C.S103{}: (C.S103{}, {==})
    case C.S104{}: (C.S104{}, {==})
    case C.S105{}: (C.S105{}, {==})
    case C.S106{}: (C.S106{}, {==})
    case C.S107{}: (C.S107{}, {==})
    case C.S108{}: (C.S108{}, {==})
    case C.S109{}: (C.S109{}, {==})
    case C.S110{}: (C.S110{}, {==})
    case C.S111{}: (C.S111{}, {==})
    case C.S112{}: (C.S112{}, {==})
    case C.S113{}: (C.S113{}, {==})
    case C.S114{}: (C.S114{}, {==})
    case C.S115{}: (C.S115{}, {==})
    case C.S116{}: (C.S116{}, {==})
    case C.S117{}: (C.S117{}, {==})
    case C.S118{}: (C.S118{}, {==})
    case C.S119{}: (C.S119{}, {==})
    case C.S120{}: (C.S120{}, {==})
    case C.S121{}: (C.S121{}, {==})
    case C.S122{}: (C.S122{}, {==})
    case C.S123{}: (C.S123{}, {==})
    case C.S124{}: (C.S124{}, {==})
    case C.S125{}: (C.S125{}, {==})
    case C.S126{}: (C.S126{}, {==})

def edgeRemoteFinishes(i: C.Index) -> C.Invariant(P.next(P.fixed(), P.RemoteFinishes{}, C.decode(i))):
  match i:
    case C.S0{}: (C.S3{}, {==})
    case C.S1{}: (C.S8{}, {==})
    case C.S2{}: (C.S3{}, {==})
    case C.S3{}: (C.S3{}, {==})
    case C.S4{}: (C.S4{}, {==})
    case C.S5{}: (C.S5{}, {==})
    case C.S6{}: (C.S20{}, {==})
    case C.S7{}: (C.S8{}, {==})
    case C.S8{}: (C.S8{}, {==})
    case C.S9{}: (C.S9{}, {==})
    case C.S10{}: (C.S27{}, {==})
    case C.S11{}: (C.S11{}, {==})
    case C.S12{}: (C.S14{}, {==})
    case C.S13{}: (C.S16{}, {==})
    case C.S14{}: (C.S14{}, {==})
    case C.S15{}: (C.S15{}, {==})
    case C.S16{}: (C.S16{}, {==})
    case C.S17{}: (C.S17{}, {==})
    case C.S18{}: (C.S18{}, {==})
    case C.S19{}: (C.S20{}, {==})
    case C.S20{}: (C.S20{}, {==})
    case C.S21{}: (C.S21{}, {==})
    case C.S22{}: (C.S43{}, {==})
    case C.S23{}: (C.S25{}, {==})
    case C.S24{}: (C.S27{}, {==})
    case C.S25{}: (C.S25{}, {==})
    case C.S26{}: (C.S26{}, {==})
    case C.S27{}: (C.S27{}, {==})
    case C.S28{}: (C.S28{}, {==})
    case C.S29{}: (C.S29{}, {==})
    case C.S30{}: (C.S30{}, {==})
    case C.S31{}: (C.S56{}, {==})
    case C.S32{}: (C.S35{}, {==})
    case C.S33{}: (C.S37{}, {==})
    case C.S34{}: (C.S34{}, {==})
    case C.S35{}: (C.S35{}, {==})
    case C.S36{}: (C.S36{}, {==})
    case C.S37{}: (C.S37{}, {==})
    case C.S38{}: (C.S38{}, {==})
    case C.S39{}: (C.S39{}, {==})
    case C.S40{}: (C.S40{}, {==})
    case C.S41{}: (C.S43{}, {==})
    case C.S42{}: (C.S42{}, {==})
    case C.S43{}: (C.S43{}, {==})
    case C.S44{}: (C.S44{}, {==})
    case C.S45{}: (C.S67{}, {==})
    case C.S46{}: (C.S49{}, {==})
    case C.S47{}: (C.S51{}, {==})
    case C.S48{}: (C.S48{}, {==})
    case C.S49{}: (C.S49{}, {==})
    case C.S50{}: (C.S50{}, {==})
    case C.S51{}: (C.S51{}, {==})
    case C.S52{}: (C.S52{}, {==})
    case C.S53{}: (C.S53{}, {==})
    case C.S54{}: (C.S77{}, {==})
    case C.S55{}: (C.S56{}, {==})
    case C.S56{}: (C.S56{}, {==})
    case C.S57{}: (C.S57{}, {==})
    case C.S58{}: (C.S62{}, {==})
    case C.S59{}: (C.S59{}, {==})
    case C.S60{}: (C.S60{}, {==})
    case C.S61{}: (C.S61{}, {==})
    case C.S62{}: (C.S62{}, {==})
    case C.S63{}: (C.S63{}, {==})
    case C.S64{}: (C.S64{}, {==})
    case C.S65{}: (C.S67{}, {==})
    case C.S66{}: (C.S66{}, {==})
    case C.S67{}: (C.S67{}, {==})
    case C.S68{}: (C.S68{}, {==})
    case C.S69{}: (C.S69{}, {==})
    case C.S70{}: (C.S73{}, {==})
    case C.S71{}: (C.S71{}, {==})
    case C.S72{}: (C.S72{}, {==})
    case C.S73{}: (C.S73{}, {==})
    case C.S74{}: (C.S74{}, {==})
    case C.S75{}: (C.S75{}, {==})
    case C.S76{}: (C.S77{}, {==})
    case C.S77{}: (C.S77{}, {==})
    case C.S78{}: (C.S78{}, {==})
    case C.S79{}: (C.S81{}, {==})
    case C.S80{}: (C.S83{}, {==})
    case C.S81{}: (C.S81{}, {==})
    case C.S82{}: (C.S82{}, {==})
    case C.S83{}: (C.S83{}, {==})
    case C.S84{}: (C.S84{}, {==})
    case C.S85{}: (C.S85{}, {==})
    case C.S86{}: (C.S86{}, {==})
    case C.S87{}: (C.S87{}, {==})
    case C.S88{}: (C.S88{}, {==})
    case C.S89{}: (C.S89{}, {==})
    case C.S90{}: (C.S92{}, {==})
    case C.S91{}: (C.S91{}, {==})
    case C.S92{}: (C.S92{}, {==})
    case C.S93{}: (C.S93{}, {==})
    case C.S94{}: (C.S111{}, {==})
    case C.S95{}: (C.S95{}, {==})
    case C.S96{}: (C.S96{}, {==})
    case C.S97{}: (C.S97{}, {==})
    case C.S98{}: (C.S100{}, {==})
    case C.S99{}: (C.S99{}, {==})
    case C.S100{}: (C.S100{}, {==})
    case C.S101{}: (C.S101{}, {==})
    case C.S102{}: (C.S102{}, {==})
    case C.S103{}: (C.S103{}, {==})
    case C.S104{}: (C.S104{}, {==})
    case C.S105{}: (C.S105{}, {==})
    case C.S106{}: (C.S106{}, {==})
    case C.S107{}: (C.S107{}, {==})
    case C.S108{}: (C.S108{}, {==})
    case C.S109{}: (C.S109{}, {==})
    case C.S110{}: (C.S111{}, {==})
    case C.S111{}: (C.S111{}, {==})
    case C.S112{}: (C.S112{}, {==})
    case C.S113{}: (C.S113{}, {==})
    case C.S114{}: (C.S114{}, {==})
    case C.S115{}: (C.S115{}, {==})
    case C.S116{}: (C.S116{}, {==})
    case C.S117{}: (C.S117{}, {==})
    case C.S118{}: (C.S118{}, {==})
    case C.S119{}: (C.S119{}, {==})
    case C.S120{}: (C.S122{}, {==})
    case C.S121{}: (C.S121{}, {==})
    case C.S122{}: (C.S122{}, {==})
    case C.S123{}: (C.S123{}, {==})
    case C.S124{}: (C.S124{}, {==})
    case C.S125{}: (C.S125{}, {==})
    case C.S126{}: (C.S126{}, {==})

def edgeHandlerCloses(i: C.Index) -> C.Invariant(P.next(P.fixed(), P.HandlerCloses{}, C.decode(i))):
  match i:
    case C.S0{}: (C.S4{}, {==})
    case C.S1{}: (C.S9{}, {==})
    case C.S2{}: (C.S4{}, {==})
    case C.S3{}: (C.S15{}, {==})
    case C.S4{}: (C.S4{}, {==})
    case C.S5{}: (C.S5{}, {==})
    case C.S6{}: (C.S21{}, {==})
    case C.S7{}: (C.S9{}, {==})
    case C.S8{}: (C.S26{}, {==})
    case C.S9{}: (C.S9{}, {==})
    case C.S10{}: (C.S29{}, {==})
    case C.S11{}: (C.S11{}, {==})
    case C.S12{}: (C.S17{}, {==})
    case C.S13{}: (C.S18{}, {==})
    case C.S14{}: (C.S36{}, {==})
    case C.S15{}: (C.S15{}, {==})
    case C.S16{}: (C.S38{}, {==})
    case C.S17{}: (C.S17{}, {==})
    case C.S18{}: (C.S18{}, {==})
    case C.S19{}: (C.S21{}, {==})
    case C.S20{}: (C.S42{}, {==})
    case C.S21{}: (C.S21{}, {==})
    case C.S22{}: (C.S44{}, {==})
    case C.S23{}: (C.S28{}, {==})
    case C.S24{}: (C.S29{}, {==})
    case C.S25{}: (C.S48{}, {==})
    case C.S26{}: (C.S26{}, {==})
    case C.S27{}: (C.S50{}, {==})
    case C.S28{}: (C.S28{}, {==})
    case C.S29{}: (C.S29{}, {==})
    case C.S30{}: (C.S30{}, {==})
    case C.S31{}: (C.S57{}, {==})
    case C.S32{}: (C.S39{}, {==})
    case C.S33{}: (C.S40{}, {==})
    case C.S34{}: (C.S34{}, {==})
    case C.S35{}: (C.S61{}, {==})
    case C.S36{}: (C.S36{}, {==})
    case C.S37{}: (C.S63{}, {==})
    case C.S38{}: (C.S38{}, {==})
    case C.S39{}: (C.S39{}, {==})
    case C.S40{}: (C.S40{}, {==})
    case C.S41{}: (C.S44{}, {==})
    case C.S42{}: (C.S42{}, {==})
    case C.S43{}: (C.S66{}, {==})
    case C.S44{}: (C.S44{}, {==})
    case C.S45{}: (C.S68{}, {==})
    case C.S46{}: (C.S52{}, {==})
    case C.S47{}: (C.S53{}, {==})
    case C.S48{}: (C.S48{}, {==})
    case C.S49{}: (C.S72{}, {==})
    case C.S50{}: (C.S50{}, {==})
    case C.S51{}: (C.S74{}, {==})
    case C.S52{}: (C.S52{}, {==})
    case C.S53{}: (C.S53{}, {==})
    case C.S54{}: (C.S78{}, {==})
    case C.S55{}: (C.S57{}, {==})
    case C.S56{}: (C.S82{}, {==})
    case C.S57{}: (C.S57{}, {==})
    case C.S58{}: (C.S64{}, {==})
    case C.S59{}: (C.S59{}, {==})
    case C.S60{}: (C.S88{}, {==})
    case C.S61{}: (C.S61{}, {==})
    case C.S62{}: (C.S89{}, {==})
    case C.S63{}: (C.S63{}, {==})
    case C.S64{}: (C.S64{}, {==})
    case C.S65{}: (C.S68{}, {==})
    case C.S66{}: (C.S66{}, {==})
    case C.S67{}: (C.S91{}, {==})
    case C.S68{}: (C.S68{}, {==})
    case C.S69{}: (C.S69{}, {==})
    case C.S70{}: (C.S75{}, {==})
    case C.S71{}: (C.S71{}, {==})
    case C.S72{}: (C.S72{}, {==})
    case C.S73{}: (C.S96{}, {==})
    case C.S74{}: (C.S74{}, {==})
    case C.S75{}: (C.S75{}, {==})
    case C.S76{}: (C.S78{}, {==})
    case C.S77{}: (C.S97{}, {==})
    case C.S78{}: (C.S78{}, {==})
    case C.S79{}: (C.S84{}, {==})
    case C.S80{}: (C.S85{}, {==})
    case C.S81{}: (C.S99{}, {==})
    case C.S82{}: (C.S82{}, {==})
    case C.S83{}: (C.S101{}, {==})
    case C.S84{}: (C.S84{}, {==})
    case C.S85{}: (C.S85{}, {==})
    case C.S86{}: (C.S104{}, {==})
    case C.S87{}: (C.S106{}, {==})
    case C.S88{}: (C.S88{}, {==})
    case C.S89{}: (C.S89{}, {==})
    case C.S90{}: (C.S93{}, {==})
    case C.S91{}: (C.S91{}, {==})
    case C.S92{}: (C.S109{}, {==})
    case C.S93{}: (C.S93{}, {==})
    case C.S94{}: (C.S112{}, {==})
    case C.S95{}: (C.S113{}, {==})
    case C.S96{}: (C.S96{}, {==})
    case C.S97{}: (C.S97{}, {==})
    case C.S98{}: (C.S102{}, {==})
    case C.S99{}: (C.S99{}, {==})
    case C.S100{}: (C.S115{}, {==})
    case C.S101{}: (C.S101{}, {==})
    case C.S102{}: (C.S102{}, {==})
    case C.S103{}: (C.S116{}, {==})
    case C.S104{}: (C.S104{}, {==})
    case C.S105{}: (C.S117{}, {==})
    case C.S106{}: (C.S106{}, {==})
    case C.S107{}: (C.S118{}, {==})
    case C.S108{}: (C.S108{}, {==})
    case C.S109{}: (C.S109{}, {==})
    case C.S110{}: (C.S112{}, {==})
    case C.S111{}: (C.S121{}, {==})
    case C.S112{}: (C.S112{}, {==})
    case C.S113{}: (C.S113{}, {==})
    case C.S114{}: (C.S124{}, {==})
    case C.S115{}: (C.S115{}, {==})
    case C.S116{}: (C.S116{}, {==})
    case C.S117{}: (C.S117{}, {==})
    case C.S118{}: (C.S118{}, {==})
    case C.S119{}: (C.S125{}, {==})
    case C.S120{}: (C.S123{}, {==})
    case C.S121{}: (C.S121{}, {==})
    case C.S122{}: (C.S126{}, {==})
    case C.S123{}: (C.S123{}, {==})
    case C.S124{}: (C.S124{}, {==})
    case C.S125{}: (C.S125{}, {==})
    case C.S126{}: (C.S126{}, {==})

def edgePersist(i: C.Index) -> C.Invariant(P.next(P.fixed(), P.Persist{}, C.decode(i))):
  match i:
    case C.S0{}: (C.S0{}, {==})
    case C.S1{}: (C.S10{}, {==})
    case C.S2{}: (C.S13{}, {==})
    case C.S3{}: (C.S16{}, {==})
    case C.S4{}: (C.S18{}, {==})
    case C.S5{}: (C.S5{}, {==})
    case C.S6{}: (C.S22{}, {==})
    case C.S7{}: (C.S24{}, {==})
    case C.S8{}: (C.S27{}, {==})
    case C.S9{}: (C.S29{}, {==})
    case C.S10{}: (C.S10{}, {==})
    case C.S11{}: (C.S11{}, {==})
    case C.S12{}: (C.S33{}, {==})
    case C.S13{}: (C.S13{}, {==})
    case C.S14{}: (C.S37{}, {==})
    case C.S15{}: (C.S38{}, {==})
    case C.S16{}: (C.S16{}, {==})
    case C.S17{}: (C.S40{}, {==})
    case C.S18{}: (C.S18{}, {==})
    case C.S19{}: (C.S41{}, {==})
    case C.S20{}: (C.S43{}, {==})
    case C.S21{}: (C.S44{}, {==})
    case C.S22{}: (C.S45{}, {==})
    case C.S23{}: (C.S46{}, {==})
    case C.S24{}: (C.S47{}, {==})
    case C.S25{}: (C.S49{}, {==})
    case C.S26{}: (C.S50{}, {==})
    case C.S27{}: (C.S51{}, {==})
    case C.S28{}: (C.S52{}, {==})
    case C.S29{}: (C.S53{}, {==})
    case C.S30{}: (C.S30{}, {==})
    case C.S31{}: (C.S31{}, {==})
    case C.S32{}: (C.S58{}, {==})
    case C.S33{}: (C.S47{}, {==})
    case C.S34{}: (C.S34{}, {==})
    case C.S35{}: (C.S62{}, {==})
    case C.S36{}: (C.S63{}, {==})
    case C.S37{}: (C.S51{}, {==})
    case C.S38{}: (C.S38{}, {==})
    case C.S39{}: (C.S64{}, {==})
    case C.S40{}: (C.S53{}, {==})
    case C.S41{}: (C.S65{}, {==})
    case C.S42{}: (C.S66{}, {==})
    case C.S43{}: (C.S67{}, {==})
    case C.S44{}: (C.S68{}, {==})
    case C.S45{}: (C.S45{}, {==})
    case C.S46{}: (C.S70{}, {==})
    case C.S47{}: (C.S47{}, {==})
    case C.S48{}: (C.S72{}, {==})
    case C.S49{}: (C.S73{}, {==})
    case C.S50{}: (C.S74{}, {==})
    case C.S51{}: (C.S51{}, {==})
    case C.S52{}: (C.S75{}, {==})
    case C.S53{}: (C.S53{}, {==})
    case C.S54{}: (C.S45{}, {==})
    case C.S55{}: (C.S80{}, {==})
    case C.S56{}: (C.S83{}, {==})
    case C.S57{}: (C.S85{}, {==})
    case C.S58{}: (C.S70{}, {==})
    case C.S59{}: (C.S59{}, {==})
    case C.S60{}: (C.S60{}, {==})
    case C.S61{}: (C.S89{}, {==})
    case C.S62{}: (C.S73{}, {==})
    case C.S63{}: (C.S74{}, {==})
    case C.S64{}: (C.S75{}, {==})
    case C.S65{}: (C.S90{}, {==})
    case C.S66{}: (C.S91{}, {==})
    case C.S67{}: (C.S92{}, {==})
    case C.S68{}: (C.S93{}, {==})
    case C.S69{}: (C.S69{}, {==})
    case C.S70{}: (C.S90{}, {==})
    case C.S71{}: (C.S71{}, {==})
    case C.S72{}: (C.S96{}, {==})
    case C.S73{}: (C.S92{}, {==})
    case C.S74{}: (C.S74{}, {==})
    case C.S75{}: (C.S93{}, {==})
    case C.S76{}: (C.S65{}, {==})
    case C.S77{}: (C.S67{}, {==})
    case C.S78{}: (C.S68{}, {==})
    case C.S79{}: (C.S98{}, {==})
    case C.S80{}: (C.S80{}, {==})
    case C.S81{}: (C.S100{}, {==})
    case C.S82{}: (C.S101{}, {==})
    case C.S83{}: (C.S83{}, {==})
    case C.S84{}: (C.S102{}, {==})
    case C.S85{}: (C.S85{}, {==})
    case C.S86{}: (C.S86{}, {==})
    case C.S87{}: (C.S107{}, {==})
    case C.S88{}: (C.S88{}, {==})
    case C.S89{}: (C.S96{}, {==})
    case C.S90{}: (C.S90{}, {==})
    case C.S91{}: (C.S109{}, {==})
    case C.S92{}: (C.S92{}, {==})
    case C.S93{}: (C.S93{}, {==})
    case C.S94{}: (C.S94{}, {==})
    case C.S95{}: (C.S114{}, {==})
    case C.S96{}: (C.S109{}, {==})
    case C.S97{}: (C.S91{}, {==})
    case C.S98{}: (C.S90{}, {==})
    case C.S99{}: (C.S115{}, {==})
    case C.S100{}: (C.S92{}, {==})
    case C.S101{}: (C.S101{}, {==})
    case C.S102{}: (C.S93{}, {==})
    case C.S103{}: (C.S114{}, {==})
    case C.S104{}: (C.S104{}, {==})
    case C.S105{}: (C.S95{}, {==})
    case C.S106{}: (C.S118{}, {==})
    case C.S107{}: (C.S107{}, {==})
    case C.S108{}: (C.S108{}, {==})
    case C.S109{}: (C.S109{}, {==})
    case C.S110{}: (C.S120{}, {==})
    case C.S111{}: (C.S122{}, {==})
    case C.S112{}: (C.S123{}, {==})
    case C.S113{}: (C.S124{}, {==})
    case C.S114{}: (C.S114{}, {==})
    case C.S115{}: (C.S109{}, {==})
    case C.S116{}: (C.S124{}, {==})
    case C.S117{}: (C.S113{}, {==})
    case C.S118{}: (C.S118{}, {==})
    case C.S119{}: (C.S119{}, {==})
    case C.S120{}: (C.S120{}, {==})
    case C.S121{}: (C.S126{}, {==})
    case C.S122{}: (C.S122{}, {==})
    case C.S123{}: (C.S123{}, {==})
    case C.S124{}: (C.S124{}, {==})
    case C.S125{}: (C.S125{}, {==})
    case C.S126{}: (C.S126{}, {==})

def edgeCrash(i: C.Index) -> C.Invariant(P.next(P.fixed(), P.Crash{}, C.decode(i))):
  match i:
    case C.S0{}: (C.S5{}, {==})
    case C.S1{}: (C.S11{}, {==})
    case C.S2{}: (C.S5{}, {==})
    case C.S3{}: (C.S5{}, {==})
    case C.S4{}: (C.S5{}, {==})
    case C.S5{}: (C.S5{}, {==})
    case C.S6{}: (C.S11{}, {==})
    case C.S7{}: (C.S11{}, {==})
    case C.S8{}: (C.S11{}, {==})
    case C.S9{}: (C.S11{}, {==})
    case C.S10{}: (C.S30{}, {==})
    case C.S11{}: (C.S11{}, {==})
    case C.S12{}: (C.S11{}, {==})
    case C.S13{}: (C.S34{}, {==})
    case C.S14{}: (C.S11{}, {==})
    case C.S15{}: (C.S5{}, {==})
    case C.S16{}: (C.S34{}, {==})
    case C.S17{}: (C.S11{}, {==})
    case C.S18{}: (C.S34{}, {==})
    case C.S19{}: (C.S11{}, {==})
    case C.S20{}: (C.S11{}, {==})
    case C.S21{}: (C.S11{}, {==})
    case C.S22{}: (C.S30{}, {==})
    case C.S23{}: (C.S11{}, {==})
    case C.S24{}: (C.S30{}, {==})
    case C.S25{}: (C.S11{}, {==})
    case C.S26{}: (C.S11{}, {==})
    case C.S27{}: (C.S30{}, {==})
    case C.S28{}: (C.S11{}, {==})
    case C.S29{}: (C.S30{}, {==})
    case C.S30{}: (C.S30{}, {==})
    case C.S31{}: (C.S11{}, {==})
    case C.S32{}: (C.S11{}, {==})
    case C.S33{}: (C.S59{}, {==})
    case C.S34{}: (C.S34{}, {==})
    case C.S35{}: (C.S11{}, {==})
    case C.S36{}: (C.S11{}, {==})
    case C.S37{}: (C.S59{}, {==})
    case C.S38{}: (C.S34{}, {==})
    case C.S39{}: (C.S11{}, {==})
    case C.S40{}: (C.S59{}, {==})
    case C.S41{}: (C.S30{}, {==})
    case C.S42{}: (C.S11{}, {==})
    case C.S43{}: (C.S30{}, {==})
    case C.S44{}: (C.S30{}, {==})
    case C.S45{}: (C.S69{}, {==})
    case C.S46{}: (C.S30{}, {==})
    case C.S47{}: (C.S71{}, {==})
    case C.S48{}: (C.S11{}, {==})
    case C.S49{}: (C.S30{}, {==})
    case C.S50{}: (C.S30{}, {==})
    case C.S51{}: (C.S71{}, {==})
    case C.S52{}: (C.S30{}, {==})
    case C.S53{}: (C.S71{}, {==})
    case C.S54{}: (C.S11{}, {==})
    case C.S55{}: (C.S11{}, {==})
    case C.S56{}: (C.S11{}, {==})
    case C.S57{}: (C.S11{}, {==})
    case C.S58{}: (C.S59{}, {==})
    case C.S59{}: (C.S59{}, {==})
    case C.S60{}: (C.S34{}, {==})
    case C.S61{}: (C.S11{}, {==})
    case C.S62{}: (C.S59{}, {==})
    case C.S63{}: (C.S59{}, {==})
    case C.S64{}: (C.S59{}, {==})
    case C.S65{}: (C.S69{}, {==})
    case C.S66{}: (C.S30{}, {==})
    case C.S67{}: (C.S69{}, {==})
    case C.S68{}: (C.S69{}, {==})
    case C.S69{}: (C.S69{}, {==})
    case C.S70{}: (C.S71{}, {==})
    case C.S71{}: (C.S71{}, {==})
    case C.S72{}: (C.S30{}, {==})
    case C.S73{}: (C.S71{}, {==})
    case C.S74{}: (C.S71{}, {==})
    case C.S75{}: (C.S71{}, {==})
    case C.S76{}: (C.S11{}, {==})
    case C.S77{}: (C.S11{}, {==})
    case C.S78{}: (C.S11{}, {==})
    case C.S79{}: (C.S11{}, {==})
    case C.S80{}: (C.S59{}, {==})
    case C.S81{}: (C.S11{}, {==})
    case C.S82{}: (C.S11{}, {==})
    case C.S83{}: (C.S59{}, {==})
    case C.S84{}: (C.S11{}, {==})
    case C.S85{}: (C.S59{}, {==})
    case C.S86{}: (C.S59{}, {==})
    case C.S87{}: (C.S59{}, {==})
    case C.S88{}: (C.S34{}, {==})
    case C.S89{}: (C.S59{}, {==})
    case C.S90{}: (C.S108{}, {==})
    case C.S91{}: (C.S69{}, {==})
    case C.S92{}: (C.S108{}, {==})
    case C.S93{}: (C.S108{}, {==})
    case C.S94{}: (C.S69{}, {==})
    case C.S95{}: (C.S71{}, {==})
    case C.S96{}: (C.S71{}, {==})
    case C.S97{}: (C.S11{}, {==})
    case C.S98{}: (C.S59{}, {==})
    case C.S99{}: (C.S11{}, {==})
    case C.S100{}: (C.S59{}, {==})
    case C.S101{}: (C.S59{}, {==})
    case C.S102{}: (C.S59{}, {==})
    case C.S103{}: (C.S59{}, {==})
    case C.S104{}: (C.S59{}, {==})
    case C.S105{}: (C.S59{}, {==})
    case C.S106{}: (C.S59{}, {==})
    case C.S107{}: (C.S71{}, {==})
    case C.S108{}: (C.S108{}, {==})
    case C.S109{}: (C.S108{}, {==})
    case C.S110{}: (C.S69{}, {==})
    case C.S111{}: (C.S69{}, {==})
    case C.S112{}: (C.S69{}, {==})
    case C.S113{}: (C.S71{}, {==})
    case C.S114{}: (C.S108{}, {==})
    case C.S115{}: (C.S59{}, {==})
    case C.S116{}: (C.S59{}, {==})
    case C.S117{}: (C.S59{}, {==})
    case C.S118{}: (C.S71{}, {==})
    case C.S119{}: (C.S108{}, {==})
    case C.S120{}: (C.S108{}, {==})
    case C.S121{}: (C.S69{}, {==})
    case C.S122{}: (C.S108{}, {==})
    case C.S123{}: (C.S108{}, {==})
    case C.S124{}: (C.S108{}, {==})
    case C.S125{}: (C.S108{}, {==})
    case C.S126{}: (C.S108{}, {==})

def edgeResume(i: C.Index) -> C.Invariant(P.next(P.fixed(), P.Resume{}, C.decode(i))):
  match i:
    case C.S0{}: (C.S0{}, {==})
    case C.S1{}: (C.S1{}, {==})
    case C.S2{}: (C.S2{}, {==})
    case C.S3{}: (C.S3{}, {==})
    case C.S4{}: (C.S4{}, {==})
    case C.S5{}: (C.S0{}, {==})
    case C.S6{}: (C.S6{}, {==})
    case C.S7{}: (C.S7{}, {==})
    case C.S8{}: (C.S8{}, {==})
    case C.S9{}: (C.S9{}, {==})
    case C.S10{}: (C.S10{}, {==})
    case C.S11{}: (C.S31{}, {==})
    case C.S12{}: (C.S12{}, {==})
    case C.S13{}: (C.S13{}, {==})
    case C.S14{}: (C.S14{}, {==})
    case C.S15{}: (C.S15{}, {==})
    case C.S16{}: (C.S16{}, {==})
    case C.S17{}: (C.S17{}, {==})
    case C.S18{}: (C.S18{}, {==})
    case C.S19{}: (C.S19{}, {==})
    case C.S20{}: (C.S20{}, {==})
    case C.S21{}: (C.S21{}, {==})
    case C.S22{}: (C.S22{}, {==})
    case C.S23{}: (C.S23{}, {==})
    case C.S24{}: (C.S24{}, {==})
    case C.S25{}: (C.S25{}, {==})
    case C.S26{}: (C.S26{}, {==})
    case C.S27{}: (C.S27{}, {==})
    case C.S28{}: (C.S28{}, {==})
    case C.S29{}: (C.S29{}, {==})
    case C.S30{}: (C.S22{}, {==})
    case C.S31{}: (C.S31{}, {==})
    case C.S32{}: (C.S32{}, {==})
    case C.S33{}: (C.S33{}, {==})
    case C.S34{}: (C.S60{}, {==})
    case C.S35{}: (C.S35{}, {==})
    case C.S36{}: (C.S36{}, {==})
    case C.S37{}: (C.S37{}, {==})
    case C.S38{}: (C.S38{}, {==})
    case C.S39{}: (C.S39{}, {==})
    case C.S40{}: (C.S40{}, {==})
    case C.S41{}: (C.S41{}, {==})
    case C.S42{}: (C.S42{}, {==})
    case C.S43{}: (C.S43{}, {==})
    case C.S44{}: (C.S44{}, {==})
    case C.S45{}: (C.S45{}, {==})
    case C.S46{}: (C.S46{}, {==})
    case C.S47{}: (C.S47{}, {==})
    case C.S48{}: (C.S48{}, {==})
    case C.S49{}: (C.S49{}, {==})
    case C.S50{}: (C.S50{}, {==})
    case C.S51{}: (C.S51{}, {==})
    case C.S52{}: (C.S52{}, {==})
    case C.S53{}: (C.S53{}, {==})
    case C.S54{}: (C.S54{}, {==})
    case C.S55{}: (C.S55{}, {==})
    case C.S56{}: (C.S56{}, {==})
    case C.S57{}: (C.S57{}, {==})
    case C.S58{}: (C.S58{}, {==})
    case C.S59{}: (C.S86{}, {==})
    case C.S60{}: (C.S60{}, {==})
    case C.S61{}: (C.S61{}, {==})
    case C.S62{}: (C.S62{}, {==})
    case C.S63{}: (C.S63{}, {==})
    case C.S64{}: (C.S64{}, {==})
    case C.S65{}: (C.S65{}, {==})
    case C.S66{}: (C.S66{}, {==})
    case C.S67{}: (C.S67{}, {==})
    case C.S68{}: (C.S68{}, {==})
    case C.S69{}: (C.S94{}, {==})
    case C.S70{}: (C.S70{}, {==})
    case C.S71{}: (C.S95{}, {==})
    case C.S72{}: (C.S72{}, {==})
    case C.S73{}: (C.S73{}, {==})
    case C.S74{}: (C.S74{}, {==})
    case C.S75{}: (C.S75{}, {==})
    case C.S76{}: (C.S76{}, {==})
    case C.S77{}: (C.S77{}, {==})
    case C.S78{}: (C.S78{}, {==})
    case C.S79{}: (C.S79{}, {==})
    case C.S80{}: (C.S80{}, {==})
    case C.S81{}: (C.S81{}, {==})
    case C.S82{}: (C.S82{}, {==})
    case C.S83{}: (C.S83{}, {==})
    case C.S84{}: (C.S84{}, {==})
    case C.S85{}: (C.S85{}, {==})
    case C.S86{}: (C.S86{}, {==})
    case C.S87{}: (C.S87{}, {==})
    case C.S88{}: (C.S88{}, {==})
    case C.S89{}: (C.S89{}, {==})
    case C.S90{}: (C.S90{}, {==})
    case C.S91{}: (C.S91{}, {==})
    case C.S92{}: (C.S92{}, {==})
    case C.S93{}: (C.S93{}, {==})
    case C.S94{}: (C.S94{}, {==})
    case C.S95{}: (C.S95{}, {==})
    case C.S96{}: (C.S96{}, {==})
    case C.S97{}: (C.S97{}, {==})
    case C.S98{}: (C.S98{}, {==})
    case C.S99{}: (C.S99{}, {==})
    case C.S100{}: (C.S100{}, {==})
    case C.S101{}: (C.S101{}, {==})
    case C.S102{}: (C.S102{}, {==})
    case C.S103{}: (C.S103{}, {==})
    case C.S104{}: (C.S104{}, {==})
    case C.S105{}: (C.S105{}, {==})
    case C.S106{}: (C.S106{}, {==})
    case C.S107{}: (C.S107{}, {==})
    case C.S108{}: (C.S119{}, {==})
    case C.S109{}: (C.S109{}, {==})
    case C.S110{}: (C.S110{}, {==})
    case C.S111{}: (C.S111{}, {==})
    case C.S112{}: (C.S112{}, {==})
    case C.S113{}: (C.S113{}, {==})
    case C.S114{}: (C.S114{}, {==})
    case C.S115{}: (C.S115{}, {==})
    case C.S116{}: (C.S116{}, {==})
    case C.S117{}: (C.S117{}, {==})
    case C.S118{}: (C.S118{}, {==})
    case C.S119{}: (C.S119{}, {==})
    case C.S120{}: (C.S120{}, {==})
    case C.S121{}: (C.S121{}, {==})
    case C.S122{}: (C.S122{}, {==})
    case C.S123{}: (C.S123{}, {==})
    case C.S124{}: (C.S124{}, {==})
    case C.S125{}: (C.S125{}, {==})
    case C.S126{}: (C.S126{}, {==})

def edgeStutter(i: C.Index) -> C.Invariant(P.next(P.fixed(), P.Stutter{}, C.decode(i))):
  match i:
    case C.S0{}: (C.S0{}, {==})
    case C.S1{}: (C.S1{}, {==})
    case C.S2{}: (C.S2{}, {==})
    case C.S3{}: (C.S3{}, {==})
    case C.S4{}: (C.S4{}, {==})
    case C.S5{}: (C.S5{}, {==})
    case C.S6{}: (C.S6{}, {==})
    case C.S7{}: (C.S7{}, {==})
    case C.S8{}: (C.S8{}, {==})
    case C.S9{}: (C.S9{}, {==})
    case C.S10{}: (C.S10{}, {==})
    case C.S11{}: (C.S11{}, {==})
    case C.S12{}: (C.S12{}, {==})
    case C.S13{}: (C.S13{}, {==})
    case C.S14{}: (C.S14{}, {==})
    case C.S15{}: (C.S15{}, {==})
    case C.S16{}: (C.S16{}, {==})
    case C.S17{}: (C.S17{}, {==})
    case C.S18{}: (C.S18{}, {==})
    case C.S19{}: (C.S19{}, {==})
    case C.S20{}: (C.S20{}, {==})
    case C.S21{}: (C.S21{}, {==})
    case C.S22{}: (C.S22{}, {==})
    case C.S23{}: (C.S23{}, {==})
    case C.S24{}: (C.S24{}, {==})
    case C.S25{}: (C.S25{}, {==})
    case C.S26{}: (C.S26{}, {==})
    case C.S27{}: (C.S27{}, {==})
    case C.S28{}: (C.S28{}, {==})
    case C.S29{}: (C.S29{}, {==})
    case C.S30{}: (C.S30{}, {==})
    case C.S31{}: (C.S31{}, {==})
    case C.S32{}: (C.S32{}, {==})
    case C.S33{}: (C.S33{}, {==})
    case C.S34{}: (C.S34{}, {==})
    case C.S35{}: (C.S35{}, {==})
    case C.S36{}: (C.S36{}, {==})
    case C.S37{}: (C.S37{}, {==})
    case C.S38{}: (C.S38{}, {==})
    case C.S39{}: (C.S39{}, {==})
    case C.S40{}: (C.S40{}, {==})
    case C.S41{}: (C.S41{}, {==})
    case C.S42{}: (C.S42{}, {==})
    case C.S43{}: (C.S43{}, {==})
    case C.S44{}: (C.S44{}, {==})
    case C.S45{}: (C.S45{}, {==})
    case C.S46{}: (C.S46{}, {==})
    case C.S47{}: (C.S47{}, {==})
    case C.S48{}: (C.S48{}, {==})
    case C.S49{}: (C.S49{}, {==})
    case C.S50{}: (C.S50{}, {==})
    case C.S51{}: (C.S51{}, {==})
    case C.S52{}: (C.S52{}, {==})
    case C.S53{}: (C.S53{}, {==})
    case C.S54{}: (C.S54{}, {==})
    case C.S55{}: (C.S55{}, {==})
    case C.S56{}: (C.S56{}, {==})
    case C.S57{}: (C.S57{}, {==})
    case C.S58{}: (C.S58{}, {==})
    case C.S59{}: (C.S59{}, {==})
    case C.S60{}: (C.S60{}, {==})
    case C.S61{}: (C.S61{}, {==})
    case C.S62{}: (C.S62{}, {==})
    case C.S63{}: (C.S63{}, {==})
    case C.S64{}: (C.S64{}, {==})
    case C.S65{}: (C.S65{}, {==})
    case C.S66{}: (C.S66{}, {==})
    case C.S67{}: (C.S67{}, {==})
    case C.S68{}: (C.S68{}, {==})
    case C.S69{}: (C.S69{}, {==})
    case C.S70{}: (C.S70{}, {==})
    case C.S71{}: (C.S71{}, {==})
    case C.S72{}: (C.S72{}, {==})
    case C.S73{}: (C.S73{}, {==})
    case C.S74{}: (C.S74{}, {==})
    case C.S75{}: (C.S75{}, {==})
    case C.S76{}: (C.S76{}, {==})
    case C.S77{}: (C.S77{}, {==})
    case C.S78{}: (C.S78{}, {==})
    case C.S79{}: (C.S79{}, {==})
    case C.S80{}: (C.S80{}, {==})
    case C.S81{}: (C.S81{}, {==})
    case C.S82{}: (C.S82{}, {==})
    case C.S83{}: (C.S83{}, {==})
    case C.S84{}: (C.S84{}, {==})
    case C.S85{}: (C.S85{}, {==})
    case C.S86{}: (C.S86{}, {==})
    case C.S87{}: (C.S87{}, {==})
    case C.S88{}: (C.S88{}, {==})
    case C.S89{}: (C.S89{}, {==})
    case C.S90{}: (C.S90{}, {==})
    case C.S91{}: (C.S91{}, {==})
    case C.S92{}: (C.S92{}, {==})
    case C.S93{}: (C.S93{}, {==})
    case C.S94{}: (C.S94{}, {==})
    case C.S95{}: (C.S95{}, {==})
    case C.S96{}: (C.S96{}, {==})
    case C.S97{}: (C.S97{}, {==})
    case C.S98{}: (C.S98{}, {==})
    case C.S99{}: (C.S99{}, {==})
    case C.S100{}: (C.S100{}, {==})
    case C.S101{}: (C.S101{}, {==})
    case C.S102{}: (C.S102{}, {==})
    case C.S103{}: (C.S103{}, {==})
    case C.S104{}: (C.S104{}, {==})
    case C.S105{}: (C.S105{}, {==})
    case C.S106{}: (C.S106{}, {==})
    case C.S107{}: (C.S107{}, {==})
    case C.S108{}: (C.S108{}, {==})
    case C.S109{}: (C.S109{}, {==})
    case C.S110{}: (C.S110{}, {==})
    case C.S111{}: (C.S111{}, {==})
    case C.S112{}: (C.S112{}, {==})
    case C.S113{}: (C.S113{}, {==})
    case C.S114{}: (C.S114{}, {==})
    case C.S115{}: (C.S115{}, {==})
    case C.S116{}: (C.S116{}, {==})
    case C.S117{}: (C.S117{}, {==})
    case C.S118{}: (C.S118{}, {==})
    case C.S119{}: (C.S119{}, {==})
    case C.S120{}: (C.S120{}, {==})
    case C.S121{}: (C.S121{}, {==})
    case C.S122{}: (C.S122{}, {==})
    case C.S123{}: (C.S123{}, {==})
    case C.S124{}: (C.S124{}, {==})
    case C.S125{}: (C.S125{}, {==})
    case C.S126{}: (C.S126{}, {==})

def closure(e: P.Boundary, i: C.Index) -> C.Invariant(P.next(P.fixed(), e, C.decode(i))):
  match e:
    case P.CreateErrDone{}: edgeCreateErrDone(i)
    case P.ProcessExit{}: edgeProcessExit(i)
    case P.CancelFails{}: edgeCancelFails(i)
    case P.RemoteFinishes{}: edgeRemoteFinishes(i)
    case P.HandlerCloses{}: edgeHandlerCloses(i)
    case P.Persist{}: edgePersist(i)
    case P.Crash{}: edgeCrash(i)
    case P.Resume{}: edgeResume(i)
    case P.Stutter{}: edgeStutter(i)

def indexSafe(i: C.Index) -> {P.safe(C.decode(i)) == True{} : Bool}:
  match i:
    case C.S0{}: {==}
    case C.S1{}: {==}
    case C.S2{}: {==}
    case C.S3{}: {==}
    case C.S4{}: {==}
    case C.S5{}: {==}
    case C.S6{}: {==}
    case C.S7{}: {==}
    case C.S8{}: {==}
    case C.S9{}: {==}
    case C.S10{}: {==}
    case C.S11{}: {==}
    case C.S12{}: {==}
    case C.S13{}: {==}
    case C.S14{}: {==}
    case C.S15{}: {==}
    case C.S16{}: {==}
    case C.S17{}: {==}
    case C.S18{}: {==}
    case C.S19{}: {==}
    case C.S20{}: {==}
    case C.S21{}: {==}
    case C.S22{}: {==}
    case C.S23{}: {==}
    case C.S24{}: {==}
    case C.S25{}: {==}
    case C.S26{}: {==}
    case C.S27{}: {==}
    case C.S28{}: {==}
    case C.S29{}: {==}
    case C.S30{}: {==}
    case C.S31{}: {==}
    case C.S32{}: {==}
    case C.S33{}: {==}
    case C.S34{}: {==}
    case C.S35{}: {==}
    case C.S36{}: {==}
    case C.S37{}: {==}
    case C.S38{}: {==}
    case C.S39{}: {==}
    case C.S40{}: {==}
    case C.S41{}: {==}
    case C.S42{}: {==}
    case C.S43{}: {==}
    case C.S44{}: {==}
    case C.S45{}: {==}
    case C.S46{}: {==}
    case C.S47{}: {==}
    case C.S48{}: {==}
    case C.S49{}: {==}
    case C.S50{}: {==}
    case C.S51{}: {==}
    case C.S52{}: {==}
    case C.S53{}: {==}
    case C.S54{}: {==}
    case C.S55{}: {==}
    case C.S56{}: {==}
    case C.S57{}: {==}
    case C.S58{}: {==}
    case C.S59{}: {==}
    case C.S60{}: {==}
    case C.S61{}: {==}
    case C.S62{}: {==}
    case C.S63{}: {==}
    case C.S64{}: {==}
    case C.S65{}: {==}
    case C.S66{}: {==}
    case C.S67{}: {==}
    case C.S68{}: {==}
    case C.S69{}: {==}
    case C.S70{}: {==}
    case C.S71{}: {==}
    case C.S72{}: {==}
    case C.S73{}: {==}
    case C.S74{}: {==}
    case C.S75{}: {==}
    case C.S76{}: {==}
    case C.S77{}: {==}
    case C.S78{}: {==}
    case C.S79{}: {==}
    case C.S80{}: {==}
    case C.S81{}: {==}
    case C.S82{}: {==}
    case C.S83{}: {==}
    case C.S84{}: {==}
    case C.S85{}: {==}
    case C.S86{}: {==}
    case C.S87{}: {==}
    case C.S88{}: {==}
    case C.S89{}: {==}
    case C.S90{}: {==}
    case C.S91{}: {==}
    case C.S92{}: {==}
    case C.S93{}: {==}
    case C.S94{}: {==}
    case C.S95{}: {==}
    case C.S96{}: {==}
    case C.S97{}: {==}
    case C.S98{}: {==}
    case C.S99{}: {==}
    case C.S100{}: {==}
    case C.S101{}: {==}
    case C.S102{}: {==}
    case C.S103{}: {==}
    case C.S104{}: {==}
    case C.S105{}: {==}
    case C.S106{}: {==}
    case C.S107{}: {==}
    case C.S108{}: {==}
    case C.S109{}: {==}
    case C.S110{}: {==}
    case C.S111{}: {==}
    case C.S112{}: {==}
    case C.S113{}: {==}
    case C.S114{}: {==}
    case C.S115{}: {==}
    case C.S116{}: {==}
    case C.S117{}: {==}
    case C.S118{}: {==}
    case C.S119{}: {==}
    case C.S120{}: {==}
    case C.S121{}: {==}
    case C.S122{}: {==}
    case C.S123{}: {==}
    case C.S124{}: {==}
    case C.S125{}: {==}
    case C.S126{}: {==}

def indexqueueAccounting(i: C.Index) -> {P.queueAccounting(C.decode(i)) == True{} : Bool}:
  match i:
    case C.S0{}: {==}
    case C.S1{}: {==}
    case C.S2{}: {==}
    case C.S3{}: {==}
    case C.S4{}: {==}
    case C.S5{}: {==}
    case C.S6{}: {==}
    case C.S7{}: {==}
    case C.S8{}: {==}
    case C.S9{}: {==}
    case C.S10{}: {==}
    case C.S11{}: {==}
    case C.S12{}: {==}
    case C.S13{}: {==}
    case C.S14{}: {==}
    case C.S15{}: {==}
    case C.S16{}: {==}
    case C.S17{}: {==}
    case C.S18{}: {==}
    case C.S19{}: {==}
    case C.S20{}: {==}
    case C.S21{}: {==}
    case C.S22{}: {==}
    case C.S23{}: {==}
    case C.S24{}: {==}
    case C.S25{}: {==}
    case C.S26{}: {==}
    case C.S27{}: {==}
    case C.S28{}: {==}
    case C.S29{}: {==}
    case C.S30{}: {==}
    case C.S31{}: {==}
    case C.S32{}: {==}
    case C.S33{}: {==}
    case C.S34{}: {==}
    case C.S35{}: {==}
    case C.S36{}: {==}
    case C.S37{}: {==}
    case C.S38{}: {==}
    case C.S39{}: {==}
    case C.S40{}: {==}
    case C.S41{}: {==}
    case C.S42{}: {==}
    case C.S43{}: {==}
    case C.S44{}: {==}
    case C.S45{}: {==}
    case C.S46{}: {==}
    case C.S47{}: {==}
    case C.S48{}: {==}
    case C.S49{}: {==}
    case C.S50{}: {==}
    case C.S51{}: {==}
    case C.S52{}: {==}
    case C.S53{}: {==}
    case C.S54{}: {==}
    case C.S55{}: {==}
    case C.S56{}: {==}
    case C.S57{}: {==}
    case C.S58{}: {==}
    case C.S59{}: {==}
    case C.S60{}: {==}
    case C.S61{}: {==}
    case C.S62{}: {==}
    case C.S63{}: {==}
    case C.S64{}: {==}
    case C.S65{}: {==}
    case C.S66{}: {==}
    case C.S67{}: {==}
    case C.S68{}: {==}
    case C.S69{}: {==}
    case C.S70{}: {==}
    case C.S71{}: {==}
    case C.S72{}: {==}
    case C.S73{}: {==}
    case C.S74{}: {==}
    case C.S75{}: {==}
    case C.S76{}: {==}
    case C.S77{}: {==}
    case C.S78{}: {==}
    case C.S79{}: {==}
    case C.S80{}: {==}
    case C.S81{}: {==}
    case C.S82{}: {==}
    case C.S83{}: {==}
    case C.S84{}: {==}
    case C.S85{}: {==}
    case C.S86{}: {==}
    case C.S87{}: {==}
    case C.S88{}: {==}
    case C.S89{}: {==}
    case C.S90{}: {==}
    case C.S91{}: {==}
    case C.S92{}: {==}
    case C.S93{}: {==}
    case C.S94{}: {==}
    case C.S95{}: {==}
    case C.S96{}: {==}
    case C.S97{}: {==}
    case C.S98{}: {==}
    case C.S99{}: {==}
    case C.S100{}: {==}
    case C.S101{}: {==}
    case C.S102{}: {==}
    case C.S103{}: {==}
    case C.S104{}: {==}
    case C.S105{}: {==}
    case C.S106{}: {==}
    case C.S107{}: {==}
    case C.S108{}: {==}
    case C.S109{}: {==}
    case C.S110{}: {==}
    case C.S111{}: {==}
    case C.S112{}: {==}
    case C.S113{}: {==}
    case C.S114{}: {==}
    case C.S115{}: {==}
    case C.S116{}: {==}
    case C.S117{}: {==}
    case C.S118{}: {==}
    case C.S119{}: {==}
    case C.S120{}: {==}
    case C.S121{}: {==}
    case C.S122{}: {==}
    case C.S123{}: {==}
    case C.S124{}: {==}
    case C.S125{}: {==}
    case C.S126{}: {==}

def indexdurableHistory(i: C.Index) -> {P.durableHistory(C.decode(i)) == True{} : Bool}:
  match i:
    case C.S0{}: {==}
    case C.S1{}: {==}
    case C.S2{}: {==}
    case C.S3{}: {==}
    case C.S4{}: {==}
    case C.S5{}: {==}
    case C.S6{}: {==}
    case C.S7{}: {==}
    case C.S8{}: {==}
    case C.S9{}: {==}
    case C.S10{}: {==}
    case C.S11{}: {==}
    case C.S12{}: {==}
    case C.S13{}: {==}
    case C.S14{}: {==}
    case C.S15{}: {==}
    case C.S16{}: {==}
    case C.S17{}: {==}
    case C.S18{}: {==}
    case C.S19{}: {==}
    case C.S20{}: {==}
    case C.S21{}: {==}
    case C.S22{}: {==}
    case C.S23{}: {==}
    case C.S24{}: {==}
    case C.S25{}: {==}
    case C.S26{}: {==}
    case C.S27{}: {==}
    case C.S28{}: {==}
    case C.S29{}: {==}
    case C.S30{}: {==}
    case C.S31{}: {==}
    case C.S32{}: {==}
    case C.S33{}: {==}
    case C.S34{}: {==}
    case C.S35{}: {==}
    case C.S36{}: {==}
    case C.S37{}: {==}
    case C.S38{}: {==}
    case C.S39{}: {==}
    case C.S40{}: {==}
    case C.S41{}: {==}
    case C.S42{}: {==}
    case C.S43{}: {==}
    case C.S44{}: {==}
    case C.S45{}: {==}
    case C.S46{}: {==}
    case C.S47{}: {==}
    case C.S48{}: {==}
    case C.S49{}: {==}
    case C.S50{}: {==}
    case C.S51{}: {==}
    case C.S52{}: {==}
    case C.S53{}: {==}
    case C.S54{}: {==}
    case C.S55{}: {==}
    case C.S56{}: {==}
    case C.S57{}: {==}
    case C.S58{}: {==}
    case C.S59{}: {==}
    case C.S60{}: {==}
    case C.S61{}: {==}
    case C.S62{}: {==}
    case C.S63{}: {==}
    case C.S64{}: {==}
    case C.S65{}: {==}
    case C.S66{}: {==}
    case C.S67{}: {==}
    case C.S68{}: {==}
    case C.S69{}: {==}
    case C.S70{}: {==}
    case C.S71{}: {==}
    case C.S72{}: {==}
    case C.S73{}: {==}
    case C.S74{}: {==}
    case C.S75{}: {==}
    case C.S76{}: {==}
    case C.S77{}: {==}
    case C.S78{}: {==}
    case C.S79{}: {==}
    case C.S80{}: {==}
    case C.S81{}: {==}
    case C.S82{}: {==}
    case C.S83{}: {==}
    case C.S84{}: {==}
    case C.S85{}: {==}
    case C.S86{}: {==}
    case C.S87{}: {==}
    case C.S88{}: {==}
    case C.S89{}: {==}
    case C.S90{}: {==}
    case C.S91{}: {==}
    case C.S92{}: {==}
    case C.S93{}: {==}
    case C.S94{}: {==}
    case C.S95{}: {==}
    case C.S96{}: {==}
    case C.S97{}: {==}
    case C.S98{}: {==}
    case C.S99{}: {==}
    case C.S100{}: {==}
    case C.S101{}: {==}
    case C.S102{}: {==}
    case C.S103{}: {==}
    case C.S104{}: {==}
    case C.S105{}: {==}
    case C.S106{}: {==}
    case C.S107{}: {==}
    case C.S108{}: {==}
    case C.S109{}: {==}
    case C.S110{}: {==}
    case C.S111{}: {==}
    case C.S112{}: {==}
    case C.S113{}: {==}
    case C.S114{}: {==}
    case C.S115{}: {==}
    case C.S116{}: {==}
    case C.S117{}: {==}
    case C.S118{}: {==}
    case C.S119{}: {==}
    case C.S120{}: {==}
    case C.S121{}: {==}
    case C.S122{}: {==}
    case C.S123{}: {==}
    case C.S124{}: {==}
    case C.S125{}: {==}
    case C.S126{}: {==}

def indexqueueBound(i: C.Index) -> {P.queueBound(C.decode(i)) == True{} : Bool}:
  match i:
    case C.S0{}: {==}
    case C.S1{}: {==}
    case C.S2{}: {==}
    case C.S3{}: {==}
    case C.S4{}: {==}
    case C.S5{}: {==}
    case C.S6{}: {==}
    case C.S7{}: {==}
    case C.S8{}: {==}
    case C.S9{}: {==}
    case C.S10{}: {==}
    case C.S11{}: {==}
    case C.S12{}: {==}
    case C.S13{}: {==}
    case C.S14{}: {==}
    case C.S15{}: {==}
    case C.S16{}: {==}
    case C.S17{}: {==}
    case C.S18{}: {==}
    case C.S19{}: {==}
    case C.S20{}: {==}
    case C.S21{}: {==}
    case C.S22{}: {==}
    case C.S23{}: {==}
    case C.S24{}: {==}
    case C.S25{}: {==}
    case C.S26{}: {==}
    case C.S27{}: {==}
    case C.S28{}: {==}
    case C.S29{}: {==}
    case C.S30{}: {==}
    case C.S31{}: {==}
    case C.S32{}: {==}
    case C.S33{}: {==}
    case C.S34{}: {==}
    case C.S35{}: {==}
    case C.S36{}: {==}
    case C.S37{}: {==}
    case C.S38{}: {==}
    case C.S39{}: {==}
    case C.S40{}: {==}
    case C.S41{}: {==}
    case C.S42{}: {==}
    case C.S43{}: {==}
    case C.S44{}: {==}
    case C.S45{}: {==}
    case C.S46{}: {==}
    case C.S47{}: {==}
    case C.S48{}: {==}
    case C.S49{}: {==}
    case C.S50{}: {==}
    case C.S51{}: {==}
    case C.S52{}: {==}
    case C.S53{}: {==}
    case C.S54{}: {==}
    case C.S55{}: {==}
    case C.S56{}: {==}
    case C.S57{}: {==}
    case C.S58{}: {==}
    case C.S59{}: {==}
    case C.S60{}: {==}
    case C.S61{}: {==}
    case C.S62{}: {==}
    case C.S63{}: {==}
    case C.S64{}: {==}
    case C.S65{}: {==}
    case C.S66{}: {==}
    case C.S67{}: {==}
    case C.S68{}: {==}
    case C.S69{}: {==}
    case C.S70{}: {==}
    case C.S71{}: {==}
    case C.S72{}: {==}
    case C.S73{}: {==}
    case C.S74{}: {==}
    case C.S75{}: {==}
    case C.S76{}: {==}
    case C.S77{}: {==}
    case C.S78{}: {==}
    case C.S79{}: {==}
    case C.S80{}: {==}
    case C.S81{}: {==}
    case C.S82{}: {==}
    case C.S83{}: {==}
    case C.S84{}: {==}
    case C.S85{}: {==}
    case C.S86{}: {==}
    case C.S87{}: {==}
    case C.S88{}: {==}
    case C.S89{}: {==}
    case C.S90{}: {==}
    case C.S91{}: {==}
    case C.S92{}: {==}
    case C.S93{}: {==}
    case C.S94{}: {==}
    case C.S95{}: {==}
    case C.S96{}: {==}
    case C.S97{}: {==}
    case C.S98{}: {==}
    case C.S99{}: {==}
    case C.S100{}: {==}
    case C.S101{}: {==}
    case C.S102{}: {==}
    case C.S103{}: {==}
    case C.S104{}: {==}
    case C.S105{}: {==}
    case C.S106{}: {==}
    case C.S107{}: {==}
    case C.S108{}: {==}
    case C.S109{}: {==}
    case C.S110{}: {==}
    case C.S111{}: {==}
    case C.S112{}: {==}
    case C.S113{}: {==}
    case C.S114{}: {==}
    case C.S115{}: {==}
    case C.S116{}: {==}
    case C.S117{}: {==}
    case C.S118{}: {==}
    case C.S119{}: {==}
    case C.S120{}: {==}
    case C.S121{}: {==}
    case C.S122{}: {==}
    case C.S123{}: {==}
    case C.S124{}: {==}
    case C.S125{}: {==}
    case C.S126{}: {==}

def indexSpawns(i: C.Index) -> {Nat.is_le(P.spawnCount(C.decode(i)), 1n) == True{} : Bool}:
  match i:
    case C.S0{}: {==}
    case C.S1{}: {==}
    case C.S2{}: {==}
    case C.S3{}: {==}
    case C.S4{}: {==}
    case C.S5{}: {==}
    case C.S6{}: {==}
    case C.S7{}: {==}
    case C.S8{}: {==}
    case C.S9{}: {==}
    case C.S10{}: {==}
    case C.S11{}: {==}
    case C.S12{}: {==}
    case C.S13{}: {==}
    case C.S14{}: {==}
    case C.S15{}: {==}
    case C.S16{}: {==}
    case C.S17{}: {==}
    case C.S18{}: {==}
    case C.S19{}: {==}
    case C.S20{}: {==}
    case C.S21{}: {==}
    case C.S22{}: {==}
    case C.S23{}: {==}
    case C.S24{}: {==}
    case C.S25{}: {==}
    case C.S26{}: {==}
    case C.S27{}: {==}
    case C.S28{}: {==}
    case C.S29{}: {==}
    case C.S30{}: {==}
    case C.S31{}: {==}
    case C.S32{}: {==}
    case C.S33{}: {==}
    case C.S34{}: {==}
    case C.S35{}: {==}
    case C.S36{}: {==}
    case C.S37{}: {==}
    case C.S38{}: {==}
    case C.S39{}: {==}
    case C.S40{}: {==}
    case C.S41{}: {==}
    case C.S42{}: {==}
    case C.S43{}: {==}
    case C.S44{}: {==}
    case C.S45{}: {==}
    case C.S46{}: {==}
    case C.S47{}: {==}
    case C.S48{}: {==}
    case C.S49{}: {==}
    case C.S50{}: {==}
    case C.S51{}: {==}
    case C.S52{}: {==}
    case C.S53{}: {==}
    case C.S54{}: {==}
    case C.S55{}: {==}
    case C.S56{}: {==}
    case C.S57{}: {==}
    case C.S58{}: {==}
    case C.S59{}: {==}
    case C.S60{}: {==}
    case C.S61{}: {==}
    case C.S62{}: {==}
    case C.S63{}: {==}
    case C.S64{}: {==}
    case C.S65{}: {==}
    case C.S66{}: {==}
    case C.S67{}: {==}
    case C.S68{}: {==}
    case C.S69{}: {==}
    case C.S70{}: {==}
    case C.S71{}: {==}
    case C.S72{}: {==}
    case C.S73{}: {==}
    case C.S74{}: {==}
    case C.S75{}: {==}
    case C.S76{}: {==}
    case C.S77{}: {==}
    case C.S78{}: {==}
    case C.S79{}: {==}
    case C.S80{}: {==}
    case C.S81{}: {==}
    case C.S82{}: {==}
    case C.S83{}: {==}
    case C.S84{}: {==}
    case C.S85{}: {==}
    case C.S86{}: {==}
    case C.S87{}: {==}
    case C.S88{}: {==}
    case C.S89{}: {==}
    case C.S90{}: {==}
    case C.S91{}: {==}
    case C.S92{}: {==}
    case C.S93{}: {==}
    case C.S94{}: {==}
    case C.S95{}: {==}
    case C.S96{}: {==}
    case C.S97{}: {==}
    case C.S98{}: {==}
    case C.S99{}: {==}
    case C.S100{}: {==}
    case C.S101{}: {==}
    case C.S102{}: {==}
    case C.S103{}: {==}
    case C.S104{}: {==}
    case C.S105{}: {==}
    case C.S106{}: {==}
    case C.S107{}: {==}
    case C.S108{}: {==}
    case C.S109{}: {==}
    case C.S110{}: {==}
    case C.S111{}: {==}
    case C.S112{}: {==}
    case C.S113{}: {==}
    case C.S114{}: {==}
    case C.S115{}: {==}
    case C.S116{}: {==}
    case C.S117{}: {==}
    case C.S118{}: {==}
    case C.S119{}: {==}
    case C.S120{}: {==}
    case C.S121{}: {==}
    case C.S122{}: {==}
    case C.S123{}: {==}
    case C.S124{}: {==}
    case C.S125{}: {==}
    case C.S126{}: {==}

def indexWrites(o: P.Op, i: C.Index) -> {Nat.is_le(P.terminalWrites(o, C.decode(i)), 1n) == True{} : Bool}:
  match o:
    case P.Shell{}:
      match i:
        case C.S0{}: {==}
        case C.S1{}: {==}
        case C.S2{}: {==}
        case C.S3{}: {==}
        case C.S4{}: {==}
        case C.S5{}: {==}
        case C.S6{}: {==}
        case C.S7{}: {==}
        case C.S8{}: {==}
        case C.S9{}: {==}
        case C.S10{}: {==}
        case C.S11{}: {==}
        case C.S12{}: {==}
        case C.S13{}: {==}
        case C.S14{}: {==}
        case C.S15{}: {==}
        case C.S16{}: {==}
        case C.S17{}: {==}
        case C.S18{}: {==}
        case C.S19{}: {==}
        case C.S20{}: {==}
        case C.S21{}: {==}
        case C.S22{}: {==}
        case C.S23{}: {==}
        case C.S24{}: {==}
        case C.S25{}: {==}
        case C.S26{}: {==}
        case C.S27{}: {==}
        case C.S28{}: {==}
        case C.S29{}: {==}
        case C.S30{}: {==}
        case C.S31{}: {==}
        case C.S32{}: {==}
        case C.S33{}: {==}
        case C.S34{}: {==}
        case C.S35{}: {==}
        case C.S36{}: {==}
        case C.S37{}: {==}
        case C.S38{}: {==}
        case C.S39{}: {==}
        case C.S40{}: {==}
        case C.S41{}: {==}
        case C.S42{}: {==}
        case C.S43{}: {==}
        case C.S44{}: {==}
        case C.S45{}: {==}
        case C.S46{}: {==}
        case C.S47{}: {==}
        case C.S48{}: {==}
        case C.S49{}: {==}
        case C.S50{}: {==}
        case C.S51{}: {==}
        case C.S52{}: {==}
        case C.S53{}: {==}
        case C.S54{}: {==}
        case C.S55{}: {==}
        case C.S56{}: {==}
        case C.S57{}: {==}
        case C.S58{}: {==}
        case C.S59{}: {==}
        case C.S60{}: {==}
        case C.S61{}: {==}
        case C.S62{}: {==}
        case C.S63{}: {==}
        case C.S64{}: {==}
        case C.S65{}: {==}
        case C.S66{}: {==}
        case C.S67{}: {==}
        case C.S68{}: {==}
        case C.S69{}: {==}
        case C.S70{}: {==}
        case C.S71{}: {==}
        case C.S72{}: {==}
        case C.S73{}: {==}
        case C.S74{}: {==}
        case C.S75{}: {==}
        case C.S76{}: {==}
        case C.S77{}: {==}
        case C.S78{}: {==}
        case C.S79{}: {==}
        case C.S80{}: {==}
        case C.S81{}: {==}
        case C.S82{}: {==}
        case C.S83{}: {==}
        case C.S84{}: {==}
        case C.S85{}: {==}
        case C.S86{}: {==}
        case C.S87{}: {==}
        case C.S88{}: {==}
        case C.S89{}: {==}
        case C.S90{}: {==}
        case C.S91{}: {==}
        case C.S92{}: {==}
        case C.S93{}: {==}
        case C.S94{}: {==}
        case C.S95{}: {==}
        case C.S96{}: {==}
        case C.S97{}: {==}
        case C.S98{}: {==}
        case C.S99{}: {==}
        case C.S100{}: {==}
        case C.S101{}: {==}
        case C.S102{}: {==}
        case C.S103{}: {==}
        case C.S104{}: {==}
        case C.S105{}: {==}
        case C.S106{}: {==}
        case C.S107{}: {==}
        case C.S108{}: {==}
        case C.S109{}: {==}
        case C.S110{}: {==}
        case C.S111{}: {==}
        case C.S112{}: {==}
        case C.S113{}: {==}
        case C.S114{}: {==}
        case C.S115{}: {==}
        case C.S116{}: {==}
        case C.S117{}: {==}
        case C.S118{}: {==}
        case C.S119{}: {==}
        case C.S120{}: {==}
        case C.S121{}: {==}
        case C.S122{}: {==}
        case C.S123{}: {==}
        case C.S124{}: {==}
        case C.S125{}: {==}
        case C.S126{}: {==}
    case P.Remote{}:
      match i:
        case C.S0{}: {==}
        case C.S1{}: {==}
        case C.S2{}: {==}
        case C.S3{}: {==}
        case C.S4{}: {==}
        case C.S5{}: {==}
        case C.S6{}: {==}
        case C.S7{}: {==}
        case C.S8{}: {==}
        case C.S9{}: {==}
        case C.S10{}: {==}
        case C.S11{}: {==}
        case C.S12{}: {==}
        case C.S13{}: {==}
        case C.S14{}: {==}
        case C.S15{}: {==}
        case C.S16{}: {==}
        case C.S17{}: {==}
        case C.S18{}: {==}
        case C.S19{}: {==}
        case C.S20{}: {==}
        case C.S21{}: {==}
        case C.S22{}: {==}
        case C.S23{}: {==}
        case C.S24{}: {==}
        case C.S25{}: {==}
        case C.S26{}: {==}
        case C.S27{}: {==}
        case C.S28{}: {==}
        case C.S29{}: {==}
        case C.S30{}: {==}
        case C.S31{}: {==}
        case C.S32{}: {==}
        case C.S33{}: {==}
        case C.S34{}: {==}
        case C.S35{}: {==}
        case C.S36{}: {==}
        case C.S37{}: {==}
        case C.S38{}: {==}
        case C.S39{}: {==}
        case C.S40{}: {==}
        case C.S41{}: {==}
        case C.S42{}: {==}
        case C.S43{}: {==}
        case C.S44{}: {==}
        case C.S45{}: {==}
        case C.S46{}: {==}
        case C.S47{}: {==}
        case C.S48{}: {==}
        case C.S49{}: {==}
        case C.S50{}: {==}
        case C.S51{}: {==}
        case C.S52{}: {==}
        case C.S53{}: {==}
        case C.S54{}: {==}
        case C.S55{}: {==}
        case C.S56{}: {==}
        case C.S57{}: {==}
        case C.S58{}: {==}
        case C.S59{}: {==}
        case C.S60{}: {==}
        case C.S61{}: {==}
        case C.S62{}: {==}
        case C.S63{}: {==}
        case C.S64{}: {==}
        case C.S65{}: {==}
        case C.S66{}: {==}
        case C.S67{}: {==}
        case C.S68{}: {==}
        case C.S69{}: {==}
        case C.S70{}: {==}
        case C.S71{}: {==}
        case C.S72{}: {==}
        case C.S73{}: {==}
        case C.S74{}: {==}
        case C.S75{}: {==}
        case C.S76{}: {==}
        case C.S77{}: {==}
        case C.S78{}: {==}
        case C.S79{}: {==}
        case C.S80{}: {==}
        case C.S81{}: {==}
        case C.S82{}: {==}
        case C.S83{}: {==}
        case C.S84{}: {==}
        case C.S85{}: {==}
        case C.S86{}: {==}
        case C.S87{}: {==}
        case C.S88{}: {==}
        case C.S89{}: {==}
        case C.S90{}: {==}
        case C.S91{}: {==}
        case C.S92{}: {==}
        case C.S93{}: {==}
        case C.S94{}: {==}
        case C.S95{}: {==}
        case C.S96{}: {==}
        case C.S97{}: {==}
        case C.S98{}: {==}
        case C.S99{}: {==}
        case C.S100{}: {==}
        case C.S101{}: {==}
        case C.S102{}: {==}
        case C.S103{}: {==}
        case C.S104{}: {==}
        case C.S105{}: {==}
        case C.S106{}: {==}
        case C.S107{}: {==}
        case C.S108{}: {==}
        case C.S109{}: {==}
        case C.S110{}: {==}
        case C.S111{}: {==}
        case C.S112{}: {==}
        case C.S113{}: {==}
        case C.S114{}: {==}
        case C.S115{}: {==}
        case C.S116{}: {==}
        case C.S117{}: {==}
        case C.S118{}: {==}
        case C.S119{}: {==}
        case C.S120{}: {==}
        case C.S121{}: {==}
        case C.S122{}: {==}
        case C.S123{}: {==}
        case C.S124{}: {==}
        case C.S125{}: {==}
        case C.S126{}: {==}
